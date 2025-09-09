#!/isan/bin/python
import os
import glob
import logging
import requests
import signal
import time
from datetime import datetime

from cli import cli, clid, json
from cisco import vrf
from errors import cmd_exec_error

vrf.set_global_vrf("management")

BOOTSTRAP_CONFIG = "https://localhost/poap.json"
LOCAL_IMAGE_DIR = "bootflash:///"


def generate_type8_password(password: str, salt_bytes: bytes = None) -> str:
    """
    Generate a Cisco type 8 password hash.

    Password hasing is done in accordance with
    https://media.defense.gov/2022/Feb/17/2002940795/-1/-1/1/CSI_CISCO_PASSWORD_TYPES_BEST_PRACTICES_20220217.PDF
    This means Cisco type 8 passwords aka PBKDF2, with SHA256, 20k iterations, and a 80 bit salt. The password generation
    function will be provided here but will not be used in this script.
    """
    import hashlib
    import base64
    import os

    # Non-stanard, not documented, no official code, you're right, of course it's Cisco.
    # I got these code bits from
    # https://github.com/BrettVerney/ciscoPWDhasher/blob/master/CiscoPWDhasher/__init__.py#L8-L11
    std_b64chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
    cisco_b64chars = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
    b64table = str.maketrans(std_b64chars, cisco_b64chars)

    def to_cisco_base64(data: bytes) -> str:
        """Convert bytes to Cisco base64 format."""
        b64 = base64.b64encode(data).decode('ascii')
        return b64.translate(b64table).rstrip('=')

    if not salt_bytes:
        salt_bytes = os.urandom(10)  # 80 bits = 10 bytes
    dk = hashlib.pbkdf2_hmac('sha256', password.encode(), salt_bytes, 20000)
    return f"$nx-pbkdf2${to_cisco_base64(salt_bytes)}${to_cisco_base64(dk)}"

# Expected structure of the bootstrap configuration file:
# {
#     "image": {
#         "version": "10.4(4)",
#         "url": "https://example.com/path/to/nxos64-cs.10.4.4.M.bin",
#         "md5sum "366ef574de8899b7179e75dc6856492f"
#     },
#     "config": {
#         "lines": [
#            "interface Ethernet1/1",

LOG = logging.getLogger("poap")


class ChecksumError(Exception):
    """Custom exception for checksum errors."""
    pass


def cli_json(command: str):
    return json.loads(clid(command))


def to_posix_path(path: str) -> str:
    """Convert a Cisco-style path to a POSIX-style path."""
    ppath = []
    tokens = path.split("/")
    if tokens[0] in ["bootflash:", "flash:", "nvram:", "usb2:", "usb1:"]:
        ppath.append('/' + tokens[0].replace(":", ""))
        for token in tokens[1:]:
            if token:
                ppath.append(token)
        return "/".join(ppath)
    else:
        raise ValueError(f"Unknown path type: {path}")


def setup_logging():
    streamHandler = logging.StreamHandler()
    formatter = logging.Formatter("%(asctime)s - %(name)s - %(levelname)s - %(message)s")
    streamHandler.setFormatter(formatter)
    LOG.addHandler(streamHandler)
    iso_date = datetime.now().isoformat(timespec="seconds").replace(":", "-")
    fileHandler = logging.FileHandler(f"/bootflash/poap-{iso_date}.log")
    fileHandler.setFormatter(formatter)
    LOG.addHandler(fileHandler)
    LOG.setLevel(logging.INFO)


def get_bootstrap_data(serial: str) -> dict:
    response = requests.get(BOOTSTRAP_CONFIG, params=dict(serial=serial), timeout=10)
    response.raise_for_status()
    return response.json()


def get_booted_image():
    show_version = cli_json('show version')

    installed = show_version['nxos_ver_str']
    booted_image_path = show_version['nxos_file_name']
    return {
        "version": installed,
        "path": booted_image_path,
    }


def clean_old_images(running_image_path: str, target_image_path: str):
    bin_files = glob.glob("/bootflash/*.bin", recursive=False)
    LOG.info(f"Found .bin files: {bin_files}")
    for bin_file in bin_files:
        if bin_file == target_image_path:
            LOG.info(f"Skipping target image file: {bin_file}")
            continue
        if bin_file == running_image_path:
            LOG.info(f"Skipping running image file: {bin_file}")
            continue
        try:
            LOG.info(f"Removing file: {bin_file}")
            os.remove(bin_file)
        except OSError as e:
            LOG.error(f"Failed to remove file {bin_file}: {e}")


def download_image(image_conf: dict, target_image_path: str):
    if not os.path.exists(to_posix_path(target_image_path)):
        LOG.info(f"Downloading image from {image_conf['url']} to {target_image_path}")
        response = requests.get(image_conf["url"], timeout=60, stream=True)
        response.raise_for_status()
        written = 0
        last_written = time.time()
        with open(to_posix_path(target_image_path), "wb") as f:
            for data in response.iter_content(chunk_size=50 * 1024**2):
                f.write(data)
                elapsed = time.time() - last_written
                chunk_speed = len(data) / elapsed / (1024 * 1024)
                written += len(data)
                LOG.info(f"Downloaded {written / (1024 * 1024):.2f} MB "
                         f"(avg speed over last chunk: {chunk_speed:.1f} MB/s)")
                last_written = time.time()
    else:
        LOG.info(f"Image already exists at {target_image_path}, skipping download.")
    md5sum = cli_json(f"show file {target_image_path} md5sum")['file_content_md5sum']
    if md5sum != image_conf["md5sum"]:
        raise ChecksumError(
            f"MD5 checksum mismatch for {target_image_path}. Expected {image_conf['md5sum']}, got {md5sum}")
    LOG.info(f"MD5 checksum for {target_image_path} matches expected value.")


def configure(config_lines: list):
    """Configure the device with the provided configuration lines."""
    conf_string = "configure terminal ; " + " ; ".join(config_lines) + " ; end"
    cli(conf_string)


def main():
    setup_logging()
    signal.signal(signal.SIGTERM, lambda s, f: LOG.info(f"Received signal {s}, exiting..."))

    s = os.statvfs("/bootflash/")
    freespace = s.f_bavail * s.f_frsize
    LOG.info(f"Free space on /bootflash: {freespace} bytes")

    serial = cli_json('show version')['proc_board_id']

    try:
        bootstrap = get_bootstrap_data(serial)
        LOG.info(f"Bootstrap data: {bootstrap}")
    except requests.RequestException as e:
        LOG.error(f"Failed to fetch bootstrap data: {e}")
        exit(1)

    running_image = get_booted_image()
    LOG.info(f"Currently booted image: {running_image}")
    target_image_path = f'{LOCAL_IMAGE_DIR}{os.path.basename(bootstrap["image"]["url"])}'
    clean_old_images(to_posix_path(running_image["path"]), to_posix_path(target_image_path))

    try:
        download_image(bootstrap["image"], target_image_path)
    except ChecksumError as e:
        LOG.error(e)
        if target_image_path != running_image["path"]:
            LOG.warning(f'Deleting target image {target_image_path} due to checksum error.')
            os.remove(to_posix_path(target_image_path))
            download_image(bootstrap["image"], target_image_path)
    except requests.RequestException as e:
        LOG.error(f"Failed to download image: {e}")
        exit(1)

    if running_image["path"] != target_image_path or \
       running_image["version"] != bootstrap["image"]["version"]:

        LOG.info('Setting boot image to: ' + target_image_path)
        configure([f'boot nxos {target_image_path}'])
        LOG.info("Copying running-config to startup-config.")
        cli('copy running-config startup-config')
        startup_image = cli_json('show boot')['TABLE_Startup_Bootvar']['start_image']
        LOG.info(f"Startup image set to: {startup_image}")
    else:
        LOG.info("Boot image is already set to the target image, skipping boot configuration.")

    LOG.info("Configuring device with provided configuration lines.")
    configure(bootstrap["config"]["lines"])
    try:
        cli('copy running-config startup-config')
        LOG.info("Successfully copied running-config to startup-config.")
    except cmd_exec_error as e:
        LOG.error(f"Failed to copy running-config to startup-config: {e}")
        exit(1)

    try:
        startup_config = cli('show startup-config')
        LOG.info("Startup configuration: " + startup_config)
    except cmd_exec_error as e:
        if "No startup configuration found" in str(e):
            LOG.warning("No startup configuration found, maybe we need a reboot?")
        else:
            LOG.error(f"Error showing startup-config: {e}")

    LOG.info("POAP process completed successfully")
    cli('terminal dont-ask ; reload')


if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        LOG.error(f"An uncaught error occurred: {e}")
        exit(1)
