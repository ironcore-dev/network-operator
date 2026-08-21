<!--
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0
-->

# Contributing

## Code of Conduct

All members of the project community must abide by the [SAP Open Source Code of Conduct](https://github.com/SAP/.github/blob/main/CODE_OF_CONDUCT.md).
Only by respecting each other we can develop a productive, collaborative community.
Instances of abusive, harassing, or otherwise unacceptable behavior may be reported by contacting [a project maintainer](REUSE.toml).

## Engaging in Our Project

We use GitHub to manage reviews of pull requests.

* If you are a new contributor, see: [Steps to Contribute](#steps-to-contribute)

* Before implementing your change, create an issue that describes the problem you would like to solve or the code that should be enhanced. Please note that you are willing to work on that issue.

* The team will review the issue and decide whether it should be implemented as a pull request. In that case, they will assign the issue to you. If the team decides against picking up the issue, the team will post a comment with an explanation.

## Steps to Contribute

Should you wish to work on an issue, please claim it first by commenting on the GitHub issue that you want to work on. This is to prevent duplicated efforts from other contributors on the same issue.

If you have questions about one of the issues, please comment on them, and one of the maintainers will clarify.

## Contributing Code or Documentation

You are welcome to contribute code in order to fix a bug or to implement a new feature that is logged as an issue.

The following rule governs code contributions:

* Contributions must be licensed under the [Apache 2.0 License](./LICENSE).
* Due to legal reasons, contributors will be asked to accept a Developer Certificate of Origin (DCO) when they create the first pull request to this project. This happens in an automated fashion during the submission process. SAP uses [the standard DCO text of the Linux Foundation](https://developercertificate.org/).
* Contributions must follow our [guidelines on AI-generated code](https://github.com/SAP/.github/blob/main/CONTRIBUTING_USING_GENAI.md) in case you are using such tools.

## Issues and Planning

* We use GitHub issues to track bugs and enhancement requests.

* Please provide as much context as possible when you open an issue. The information you provide must be comprehensive enough to reproduce that issue for the assignee.

## Development

This repository uses [Git LFS](https://git-lfs.com) for binary assets (images).
Run `git lfs install` once before cloning or pulling to ensure files are fetched correctly.

> **NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html).

## AI-Assisted Contributions

AI is a great accelerator, and we encourage using it. But a PR generated
with AI is still your PR. You own every line in it, the same as if you
had typed it by hand.

* **Understand every line.** If you cannot explain why a change is there
  or what it does, it is not ready to submit.
* **Double-check the model's assumptions.** AI confidently invents
  standards, API formats, field names, and protocol details that do not
  exist or are subtly wrong. Verify anything the model asserts against
  the actual spec, docs, or API.
* **Don't commit what you don't need.** Models love adding speculative
  helpers, defensive code, and future-proofing. Strip anything the
  change does not actually require.
* **Verify the testing is real.** Before you open the PR, confirm the
  tests actually exist, actually run, and actually pass.
* **Test against real APIs.** Simulated or imagined results do not
  count. If a change touches hardware, exercise it on real hardware or
  in Containerlab before review.
* **Don't outsource your verification to the reviewer.** Opening an
  unverified PR and letting the reviewer catch mistakes is
  disrespectful of their time. Come to review having already done your
  own verification.
