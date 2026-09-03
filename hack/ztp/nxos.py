#!/isan/bin/python
# SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0

#
# Example POAP (Power-On Auto Provisioning) boot script for Cisco NX-OS.
#
# This script is delivered to the switch via TFTP during POAP (see
# docs/concepts/ztp-nxos.md). It contacts the network operator's
# provisioning endpoint, downloads and verifies the NX-OS image, installs
# certificates, applies base configuration and reboots into the new image.
#
# It is provided as a reference only. Site-specific values (provisioning
# server URLs, management subnets, credentials) have been replaced with
# placeholders and must be adapted to your environment before use.
import base64
import os
import glob
import logging
import logging.handlers
from typing import List
import requests
import signal
import secrets
import time
import traceback
from datetime import datetime

from cli import cli, clid, json
from cisco import vrf

# Network operator provisioning endpoint. Replace with your own URL.
PROVISIONING_SERVER = "https://network-operator.example.com"

VERIFY_TLS = True
LOG_CONFIG = {
    "level": logging.INFO,
    "format": "%(asctime)s: %(name)s %(levelname)s - %(message)s",
    "file": True,
}

LOCAL_IMAGE_DIR = "bootflash:///"

STATIC_CONFIG = [
    "feature grpc",
    "grpc port 9339",
    "feature nxapi",
    "nxapi https port 443",
    "nxapi ssl protocols TLSv1.3",
    "feature bash-shell",
    "ssh key ecdsa 256 force",
    "no password strength-check",
    # Replace with your own hashed admin password.
    "username admin password 5 <HASHED_PASSWORD>  role network-admin",
    "hardware access-list tcam region ing-racl 1792",
    "hardware access-list tcam region ing-flow-redirect 512",
]

CLIENT_CA_CONFIG = [
    "grpc client root certificate network-operator",
]

DEVICE_CERT_CONFIG = [
    "grpc certificate device",
    "nxapi certificate trustpoint device",
]

SCRIPT_EXECUTION_STARTED = "ScriptExecutionStarted"
SCRIPT_EXECUTION_FAILED = "ScriptExecutionFailed"
INSTALLING_CERTIFICATES = "InstallingCertificates"
DOWNLOADING_IMAGE = "DownloadingImage"
IMAGE_DOWNLOAD_FAILED = "ImageDownloadFailed"
UPGRADE_STARTING = "UpgradeStarting"
UPGRADE_FAILED = "UpgradeFailed"
REBOOTING_DEVICE = "RebootingDevice"
EXECUTION_FINISHED_WITHOUT_REBOOT = "ExecutionFinishedWithoutReboot"

ALL_STATUSES = [
    SCRIPT_EXECUTION_STARTED, SCRIPT_EXECUTION_FAILED, INSTALLING_CERTIFICATES, DOWNLOADING_IMAGE,
    IMAGE_DOWNLOAD_FAILED, UPGRADE_STARTING, UPGRADE_FAILED, REBOOTING_DEVICE, EXECUTION_FINISHED_WITHOUT_REBOOT
]

# These global variables will be initialized once the script starts.
# They are needed in a lot of places.
TOKEN = None
SERIAL = None

vrf.set_global_vrf("management")
LOG = logging.getLogger("poap-nxos")
SCHEDEDULED_CONFIG = ""


def setup_logging():
    streamHandler = logging.StreamHandler()
    formatter = logging.Formatter(LOG_CONFIG["format"])
    streamHandler.setFormatter(formatter)
    LOG.addHandler(streamHandler)
    syslogHandler = logging.handlers.SysLogHandler(address="/dev/log")
    syslogHandler.setFormatter(formatter)
    LOG.addHandler(syslogHandler)
    if LOG_CONFIG["file"]:
        iso_date = datetime.now().isoformat(timespec="seconds").replace(":", "-")
        fileHandler = logging.FileHandler(f"/bootflash/poap-{iso_date}.log")
        fileHandler.setFormatter(formatter)
        LOG.addHandler(fileHandler)
    LOG.setLevel(LOG_CONFIG["level"])


def cli_json(command: str):
    return json.loads(clid(command))


def os_command(command: str):
    """Execute an OS (linux) command and return its output."""
    result = os.popen(command).read()
    return result


def configure(config_lines: list, running=False, scheduled=True):
    """Configure the device with the provided configuration lines."""
    config_lines = [line for line in config_lines if line]
    if running:
        conf_string = "configure terminal ; " + " ; ".join(config_lines) + " ; end"
        cli(conf_string)
    if scheduled:
        global SCHEDEDULED_CONFIG
        SCHEDEDULED_CONFIG += "\n".join(config_lines) + "\n"


def to_posix_path(path: str) -> str:
    """Convert a Cisco-style path to a POSIX-style path."""
    ppath = []
    tokens = path.split("/")
    if tokens[0] in ["bootflash:", "flash:", "nvram:", "usb2:", "usb1:", "volatile:"]:
        ppath.append('/' + tokens[0].replace(":", ""))
        for token in tokens[1:]:
            if token:
                ppath.append(token)
        return "/".join(ppath)
    else:
        raise ValueError(f"Unknown path type: {path}")


def get_provisioning_data() -> dict:
    url = f"{PROVISIONING_SERVER}/provisioning/config"
    response = requests.get(url, params={"serial": SERIAL}, timeout=10, verify=VERIFY_TLS)
    response.raise_for_status()
    return response.json()


def report_status(status: str, message: str = ""):
    if status not in ALL_STATUSES:
        raise ValueError(f"Cannot report status: {status}")
    payload = {
        "status": status,
    }
    if message:
        payload["detail"] = ';'.join(message.split('\n'))
    try:
        headers = {"Authorization": f"Bearer {TOKEN}"}
        response = requests.put(PROVISIONING_SERVER + "/provisioning/status-report",
                                json=payload,
                                params={"serial": SERIAL},
                                headers=headers,
                                timeout=5,
                                verify=VERIFY_TLS)
        response.raise_for_status()
    except requests.RequestException as e:
        logging.error(f"Failed to report status: {e}")


def get_client_ca() -> str:
    headers = {"Authorization": f"Bearer {TOKEN}"}
    response = requests.get(PROVISIONING_SERVER + "/provisioning/mtls-client-ca",
                            headers=headers, params={"serial": SERIAL},
                            timeout=5, verify=VERIFY_TLS)
    if response.status_code == 404:
        return None
    response.raise_for_status()
    return response.text


def configure_ca_certificates(ca_data: str = None):
    crt_path = 'volatile:///network-operator-ca.crt'
    p7_path = 'bootflash:///network-operator-ca.p7'
    if os.path.exists(to_posix_path(p7_path)):
        os.remove(to_posix_path(p7_path))
    if os.path.exists(to_posix_path(crt_path)):
        os.remove(to_posix_path(crt_path))
    with open(to_posix_path(crt_path), 'w') as f:
        f.write(ca_data)
    os_command(f"openssl crl2pkcs7 -nocrl -certfile {to_posix_path(crt_path)} -out {to_posix_path(p7_path)}")
    configure([
        'crypto ca trustpoint network-operator',
        'crypto ca import network-operator pkcs7 {} force'.format(p7_path),
        'crypto ca cabundle network-operator',])


def get_device_certificates():
    headers = {"Authorization": f"Bearer {TOKEN}"}
    response = requests.get(PROVISIONING_SERVER + "/provisioning/device-certificate",
                            headers=headers,
                            params={"serial": SERIAL},
                            timeout=5,
                            verify=VERIFY_TLS)
    if response.status_code == 404:
        return None
    return response.json()


def configure_device_certitificates(cert_data: dict = None):
    key_path = 'volatile:///device-tls.key'
    crt_path = 'volatile:///device-tls.crt'
    ca_path = 'volatile:///device-ca.crt'
    p12_path = 'bootflash:///device-cert.p12'
    for path in [key_path, crt_path, ca_path, p12_path]:
        if os.path.exists(to_posix_path(path)):
            os.remove(to_posix_path(path))
    with open(to_posix_path(key_path), 'w') as f:
        f.write(cert_data["privateKey"])
    with open(to_posix_path(crt_path), 'w') as f:
        f.write(cert_data["certificate"])
    with open(to_posix_path(ca_path), 'w') as f:
        f.write(cert_data["caCertificate"])
    password = secrets.token_hex(16)
    os_command(f"openssl pkcs12 -export -in {to_posix_path(crt_path)} -inkey {to_posix_path(key_path)} "
            f"-certfile {to_posix_path(ca_path)} -out {to_posix_path(p12_path)} "
            "-passout pass:'{}'".format(password))
    os.remove(to_posix_path(key_path))
    os.remove(to_posix_path(crt_path))
    os.remove(to_posix_path(ca_path))
    configure(['crypto ca trustpoint device',
            'crypto ca import device pkcs12 {} {}'.format(p12_path, password)])


def verify_checksum(checksum: str, checksum_type: str, path: str = None) -> bool:
    if checksum_type not in ["MD5", "SHA256", "SHA512"]:
        raise ValueError(f"Unsupported checksum type: {checksum_type}")
    nxos_checksum_type = checksum_type.lower() + "sum"

    if not path:
        path = cli_json('show version')['nxos_file_name']

    test_checksum = cli(f"show file {path} {nxos_checksum_type}").strip()
    return test_checksum == checksum


def clean_old_images():
    s = os.statvfs("/bootflash/")
    before = s.f_bavail * s.f_frsize

    bin_files = glob.glob("/bootflash/*.bin", recursive=False)
    LOG.info(f"Found .bin files: {bin_files}")

    booted_image = cli_json('show version')['nxos_file_name']
    booted_image_path = to_posix_path(booted_image)
    for bin_file in bin_files:
        if bin_file == booted_image_path:
            LOG.info(f"Skipping booted image file: {bin_file}")
            continue
        try:
            LOG.info(f"Removing file: {bin_file}")
            os.remove(bin_file)
        except OSError as e:
            LOG.error(f"Failed to remove file {bin_file}: {e}")

    s = os.statvfs("/bootflash/")
    after = s.f_bavail * s.f_frsize
    LOG.info(f"Cleaned old images, freed {(before - after)/1024**2} MB of space.")


def download_image(image_url: str, target_path: str):
    LOG.info(f"Downloading image from {image_url} to {target_path}")
    response = requests.get(image_url, timeout=60, stream=True, verify=VERIFY_TLS)
    response.raise_for_status()
    written = 0
    last_written = time.time()
    with open(to_posix_path(target_path), "wb") as f:
        for data in response.iter_content(chunk_size=50 * 1024**2):
            f.write(data)
            elapsed = time.time() - last_written
            chunk_speed = len(data) / elapsed / (1024 * 1024)
            written += len(data)
            last_written = time.time()
            msg = f"Downloaded {written / (1024 * 1024):.2f} MB (avg speed: {chunk_speed:.1f} MB/s)"
            LOG.info(msg)
            report_status(DOWNLOADING_IMAGE, msg)


def set_firmware(image_path: str = None):
    if not image_path:
        image_path = cli_json('show version')['nxos_file_name']
    LOG.info('Setting boot image to: ' + image_path)
    configure([f'boot nxos {image_path}'], running=True)
    cli('copy running-config startup-config')


def upgrade_firmware(target_path: str):
    report_status(UPGRADE_STARTING, "Starting firmware upgrade.")
    # Use `install all` rather than just `boot nxos ...`: it runs compatibility
    # checks and, since NX-OS 10.5(3), upgrades the bundled BIOS and EPLD images
    # when required. It refuses to run while `boot poap enable` is set, so POAP
    # must be disabled around the command.
    # Note: on switches affected by the Secure Boot vulnerability
    # (cisco-sa-20190513-secureboot), `install all` skips the EPLD upgrade from
    # 10.6(1)F onward; those require a manual `install epld` post-provisioning.
    configure(['no boot poap enable'], running=True, scheduled=False)
    cli('copy running-config startup-config')
    cli(f'install all nxos {target_path} no-reload')


def configure_management_networking():
    dhcp_config = cli("show run | sec '(interface mgmt0|vrf context management)'").split('\n')
    dhcp_config = [line for line in dhcp_config if line]
    configure(dhcp_config)


def configure_user_accounts(user_accounts: List[dict]):
    config = []
    hash_algo_map = {
    "Encrypt": 5,
    "Pbkdf2": 8,
    "scrypt": 9
    }
    for a in user_accounts:
        if a.get("hashedPassword") is None or a.get("hashAlgorithm") is None:
            config.append(f'username {a["username"]} role network-admin')
            continue
        algo = hash_algo_map.get(a["hashAlgorithm"])
        if not algo:
            LOG.warning(f"Unsupported hash algorithm {a['hashAlgorithm']} for user {a['username']}, skipping.")
            continue
        config.append(f'username {a["username"]} password {algo} {a["hashedPassword"]}  role network-admin')
    if len(config) == 0:
        LOG.error("No valid user accounts to configure.")
        report_status(SCRIPT_EXECUTION_FAILED, "No valid user accounts to configure.")
        exit(-1)
    configure(config)


def main():
    setup_logging()
    signal.signal(signal.SIGTERM, lambda s: LOG.info(f"Received signal {s}, exiting..."))

    global TOKEN, SERIAL
    LOG.info(f"Using provisioning server: {PROVISIONING_SERVER}")
    SERIAL = cli_json('show version')['proc_board_id']
    LOG.info(f"Starting POAP process for device with serial: {SERIAL}")
    try:
        provisioning_data = get_provisioning_data()
    except requests.RequestException as e:
        LOG.error(f"Failed to fetch provisioning data: {e}")
        exit(-1)
    TOKEN = provisioning_data.get("provisioningToken")
    if not TOKEN:
        LOG.error("Provisioning token not found in provisioning data.")
        exit(-1)

    report_status(SCRIPT_EXECUTION_STARTED, "Boot script initialized.")

    clean_old_images()

    LOG.info("Checking if firmware upgrade is needed.")
    image = provisioning_data.get("image")
    if not image:
        LOG.error("No image information provided in provisioning data.")
        report_status(UPGRADE_FAILED, "No image information for firmware upgrade.")
        exit(-1)

    # Determine if upgrade is needed and download if necessary
    need_upgrade = not verify_checksum(checksum=image["checksum"], checksum_type=image["checksumType"])
    LOG.info(f"Firmware upgrade needed: {need_upgrade}")
    if need_upgrade:
        need_download = True
        target_path = f'{LOCAL_IMAGE_DIR}{os.path.basename(image["url"])}'
        if os.path.exists(to_posix_path(target_path)):
            LOG.info(f"Image already exists at {target_path}, verifying checksum.")
            match = verify_checksum(checksum=image["checksum"], checksum_type=image["checksumType"], path=target_path)
            if match:
                LOG.info("Existing image checksum matches, skipping download.")
                need_download = False
            else:
                LOG.warning("Existing image checksum does not match, will re-download the image.")
                os.remove(to_posix_path(target_path))
        if need_download:
            report_status(DOWNLOADING_IMAGE, "Downloading firmware image.")
            try:
                download_image(image["url"], f'{LOCAL_IMAGE_DIR}{os.path.basename(image["url"])}')
            except requests.RequestException as e:
                LOG.error(f"Failed to download or verify image: {e}")
                report_status(IMAGE_DOWNLOAD_FAILED, f"Image download or verification failed: {e}")
                exit(-1)
            match = verify_checksum(checksum=image["checksum"], checksum_type=image["checksumType"], path=target_path)
            if not match:
                report_status(IMAGE_DOWNLOAD_FAILED, "Downloaded image checksum does not match.")
                LOG.error("Downloaded image checksum does not match expected value.")
                exit(-1)

    configure([f'hostname {provisioning_data["hostname"]}'])
    configure_management_networking()
    configure_user_accounts(provisioning_data.get("userAccounts", []))
    configure(STATIC_CONFIG)

    if ca_data := get_client_ca():
        report_status(INSTALLING_CERTIFICATES, "Installing CA certificates.")
        configure_ca_certificates(ca_data)
        configure(CLIENT_CA_CONFIG)

    if cert := get_device_certificates():
        report_status(INSTALLING_CERTIFICATES, "Installing device certificates.")
        configure_device_certitificates(cert)
        configure(DEVICE_CERT_CONFIG)

    if need_upgrade:
        upgrade_firmware(target_path)
    else:
        set_firmware()

    # Writing the scheduled-config directly does not reliably survive the POAP
    # reboot. Writing it to a file first and then copying it into
    # scheduled-config is the only approach found to work consistently.

    cfg_file = "bootflash:///scheduled-config.cfg"
    with open(to_posix_path(cfg_file), 'w') as f:
        f.write(SCHEDEDULED_CONFIG)
    if os.path.exists(to_posix_path('bootflash:///scheduled-config')):
        os.remove(to_posix_path('bootflash:///scheduled-config'))
    cli(f'copy {cfg_file} scheduled-config')
    os.remove(to_posix_path(cfg_file))

    # Some older reference scripts indicate POAP can complete without a reboot
    # using exit codes, but this could not be made to work and there is no
    # vendor documentation on the required behaviour.
    # reference: https://github.com/CiscoSE/Cisco-POAP/blob/master/poap.py#L3-L9
    if need_upgrade:
        report_status(REBOOTING_DEVICE, "Rebooting device to complete firmware upgrade.")
    else:
        report_status(REBOOTING_DEVICE, "Rebooting device.")


if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        e = traceback.format_exc()
        stack = traceback.format_stack()
        LOG.error(f"An uncaught error occurred: {e}")
        if TOKEN:
            report_status(SCRIPT_EXECUTION_FAILED, f"Uncaught error: {'; '.join(e.splitlines())}")
            LOG.error("Stack trace:")
            for line in stack:
                LOG.error(line)
        exit(-1)
