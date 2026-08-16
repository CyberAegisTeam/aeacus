# Aeacus 3.0.0 Studio and Release Guide

Aeacus 3.0.0 adds two author applications while preserving the participant workflow used by Aeacus 2:

- **Aeacus Studio Personal** runs on an author's Windows, macOS, or Linux computer. It creates image projects, scoring configurations, README content, forensic questions, user scripts, and replacement `scoring.dat` files.
- **Aeacus Studio Development** runs with administrator/root privileges inside the Windows or Linux VM being authored. It initializes Aeacus, imports the project, evaluates checks continuously, tracks implementation progress, and performs the traditional Aeacus release.
- **Phocus/CSSClient** remains the participant scoring runtime. Participants continue to use the existing README, Scoring Report, Team ID, Stop Scoring shortcut where supported, and forensic-question files.

## Security warning

An `.aeacus` project contains author-only information. It may contain plaintext scoring-server passwords, initial user passwords, expected forensic answers, scripts, and complete vulnerability logic. Store it like a secret. Never leave it in the released VM or send it to participants.

## Part 1: Build an official Aeacus 3.0.0 release

Build official releases from a trusted Linux development machine. Install:

- Go 1.25 or newer
- Git
- `zip`
- `tar`
- `xxd`
- GNU `sed`

Clone and enter the repository:

```bash
git clone https://github.com/elysium-suite/aeacus.git
cd aeacus
```

Run all host tests and cross-platform compile checks:

```bash
make test-v3
```

Create a matched local release for the current build host:

```bash
make release-v3
```

The build script generates one set of configuration-encryption values and compiles the native applications supported by the current build host. It restores the source crypto files automatically, even when the build fails. For an official cross-platform release, run **Build Aeacus Studio 3.0.0** in GitHub Actions; its macOS, Windows, and Linux jobs all consume one shared release key set.

Artifacts are written under:

```text
dist/aeacus-3.0.0/
├── personal/
│   ├── Aeacus-Studio-Personal-Windows.exe
│   ├── Aeacus-Studio-Personal-Linux-amd64.tar.gz
│   ├── Aeacus Studio Personal.app/
│   ├── Aeacus-Studio-Personal-macOS.zip
│   └── Aeacus-Studio-Personal-macOS.dmg
├── development/
│   ├── windows/
│   ├── linux/
│   ├── Aeacus-Studio-Development-Windows.zip
│   └── Aeacus-Studio-Development-Linux.tar.gz
├── README.md
└── AEACUS_3_RELEASE_GUIDE.md
```

Keep the Personal and Development artifacts from the same release together. Personal Studio's generated `scoring.dat` is intended for the Phocus binary built in that release.

### macOS distribution

The build applies an ad-hoc signature after assembling the complete `.app`, which protects bundle integrity but does not establish a trusted developer identity. During internal testing, macOS may require Control-click → **Open**. If Gatekeeper continues to block a verified download, remove only that application's quarantine attribute and open it again:

```bash
xattr -dr com.apple.quarantine "/Applications/Aeacus Studio Personal.app"
open "/Applications/Aeacus Studio Personal.app"
```

Only do this after comparing the download against `SHA256SUMS.txt`. Official public distribution without a warning requires an Apple Developer ID certificate and notarization; an ad-hoc signature is not a substitute for either.

### Windows distribution

The Windows executables are unsigned unless the release operator applies an Authenticode certificate. Windows SmartScreen may warn for unsigned internal builds.

## Part 2: Create an image project in Personal Studio

Launch the Personal application for the author's operating system. Studio opens in its own native desktop window. It does not launch a browser, bind a localhost port, or expose the editor to the network. Imports use the operating-system file picker and exports use native save dialogs.

### Project settings

Open **Scoring Config** and configure:

- Image name, such as `Linux_PR6`
- Round title
- Operating-system description
- Main user
- Timezone
- Sarpedon address
- Round scoring password
- Local/remote scoring behavior

The generated `scoring.conf` appears beside the visual editor. Existing TOML configurations can be imported. Aeacus 2 comment headings such as `##### USERS / GROUPS` are imported as categories. Aeacus 3 stores stable check IDs and category metadata directly.

### Vulnerabilities

Add a vulnerability, message, category, points, hint, and its conditions. Studio supports pass conditions, pass overrides, failure conditions, and raw condition fields for every existing Aeacus condition type.

Use **Regex Builder** for:

- Exact, contains, starts-with, and ends-with patterns
- Flexible whitespace
- Case-insensitive matching
- Treating spaces and underscores as equivalent
- Multiple accepted values
- Configuration `key = value` lines
- Integer equality, ranges, and greater/less comparisons

Always test generated patterns against representative lines before using them.

### README

Use **README Builder** to create headings, paragraphs, emphasis, lists, tables, and scenario text. Studio stores structured author content and exports compatible safe HTML as `ReadMe.conf`. Users marked **Add to README** are appended to the authorized administrator/user sections.

### Forensic questions

Use **Forensics** to add questions and multipart prompts. Each part may have multiple accepted answers, case sensitivity, and flexible whitespace. Studio generates:

- `FQ1.txt`, `FQ2.txt`, and so on
- An `ANSWER:` placeholder for every part
- Matching `FileContainsRegex` scoring checks

Review the generated answer patterns and assigned points.

### Users and setup scripts

Users can be entered individually or in bulk:

```text
alice, admin, temporary-password, sudo|developers
bob, user, temporary-password
```

Studio can generate missing passwords and export an author-only credential CSV. For each user, select whether to:

- Add the user to the README
- Generate a removal penalty
- Generate a password-change check
- Generate required group-membership checks

Studio produces a Linux Bash script and a Windows PowerShell script. Review generated scripts before running them. Windows creates local accounts immediately, but Windows normally creates a complete profile directory on first login.

### Save and transfer

Select **Save project** to download one file such as:

```text
linux-pr6.aeacus
```

Move this author-only file into the development VM through a trusted transfer mechanism.

### Replacement scoring.dat

To produce a correction shortly after release:

1. Open the saved project in the matching Personal Studio release.
2. Correct the scoring configuration.
3. Resolve all validation errors.
4. Select **Generate scoring.dat**.
5. Test the replacement against a retained author copy when practical.
6. Send only the replacement `.dat` through the approved distribution channel.

## Part 3: Initialize an image with Development Studio

Development Studio must be run as Administrator on Windows or root on Linux. Extract the complete Development bundle into a temporary author location inside the VM. Do not copy only the Studio executable; it expects matching `aeacus`, `phocus`, `assets`, and `misc` beside it.

Launch:

```powershell
Aeacus-Studio-Development.exe
```

or:

```bash
sudo ./aeacus-studio-development
```

Open **Implementation** and confirm the target:

- Windows: `C:\aeacus`
- Linux: `/opt/aeacus`

Select **Initialize image**. Studio creates the directory and copies the matched runtime binaries and default resources.

Import the `.aeacus` project and select **Install project files**. Studio writes:

- `scoring.conf`
- `ReadMe.conf`
- Generated setup scripts
- Optional gain/loss WAV files
- FQ files on the configured main user's desktop

Custom scoring sounds must be valid WAV files no larger than 5 MiB. They replace the existing `gain.wav` and `alarm.wav` assets.

## Part 4: Implement and test vulnerabilities

Run reviewed setup scripts from **Users & Scripts** if appropriate. Scripts execute with Development Studio's elevated privileges.

The Implementation dashboard evaluates the real VM through the author Aeacus binary. It does not submit scores to Sarpedon. Use manual evaluation or select a 5-, 10-, or 30-second refresh interval.

Every check shows:

- Current passing/failing result
- Points
- Manual author status
- Whether Studio has observed it passing
- Whether Studio has observed it failing

Recommended statuses:

- Not set up
- In progress
- Set up
- Fully tested
- Needs review
- Intentionally disabled

Studio records passing and failing observations automatically. The author retains control of the status dropdown.

The initial vulnerable image must score exactly zero. As vulnerabilities are intentionally added, conditions that award points should move from passing to failing. Correct each vulnerability temporarily to verify the passing state, then return it to the intended vulnerable state.

Configuration edits discovered during VM testing can be made directly in Development Studio. Save an updated `.aeacus` project back to the author's personal computer before release.

## Part 5: Release the image

Open **Release** and select **Validate and release image**.

Release is blocked unless:

- The configuration has no structural errors
- The actual current score is exactly `0`

Checks not observed in both states do not block release. Studio lists them and asks whether to continue.

Development Studio then invokes the existing platform release behavior:

- Encrypt `scoring.conf` into `scoring.dat`
- Generate the README and scoring report
- Create/reset `TeamID.txt`
- Configure autologin as currently implemented
- Install and start CSSClient/Phocus
- Install the existing participant desktop shortcuts
- Preserve forensic-question files
- Run the current optional cleanup prompts
- Remove plaintext configuration and author resources

The optional cleanup behavior is intentionally the same as legacy Aeacus. On Linux it can remove histories, caches, logs, and overwrite timestamps. Review whether this conflicts with intentional forensic evidence before accepting it.

After successful verification, Development Studio reports that release is complete, exits, and uses a delayed finalizer to delete its own executable. The VM remains running. Shut it down manually when all final review is complete.

## Part 6: Participant experience

Participants do not receive either Studio. Their experience remains the established Aeacus workflow:

- Enter the Team ID through the existing prompt/shortcut
- Open the existing README shortcut
- Open the existing Scoring Report shortcut
- Use Stop Scoring where currently supported
- Answer FQ files on the desktop

Phocus continues evaluating the image at randomized intervals and submitting scores to Sarpedon.

## Troubleshooting

### Development Studio cannot evaluate

Confirm that the Development bundle was kept intact and that `aeacus`/`aeacus.exe` exists inside the selected target directory. Run Studio as root/Administrator.

### Release is blocked with a nonzero score

Return to Implementation and inspect every passing check. The release error also lists checks currently awarding points or triggering penalties. Restore the intended vulnerable state until the score is exactly zero.

### scoring.dat cannot be decrypted

Confirm that Personal Studio and the deployed Phocus came from the same Aeacus 3.0.0 release artifacts. Regenerate the file with the matching Personal Studio build.

### FQ files are written to the wrong location

Confirm the main `user` field and operating-system description in Scoring Config, reinstall project files, and verify that the user's Desktop exists.

### Studio window does not open

On macOS, Control-click the unsigned application and choose **Open**. On Windows, approve the unsigned application in SmartScreen. On Linux, ensure GTK 3 and WebKitGTK 4.1 are installed and launch the application from a terminal to inspect startup errors.

## Maintainer verification checklist

Before publishing an Aeacus 3.0.0 release:

1. Run `make test-v3`.
2. Run `make release-v3`.
3. Confirm source crypto files were restored and no `.bak` files remain.
4. Launch Personal Studio on macOS, Windows, and Linux.
5. Import a representative legacy scoring configuration.
6. Export and re-import its Aeacus project.
7. Generate and decrypt/test `scoring.dat` with the matched Phocus build.
8. Initialize disposable Windows and Linux VMs with Development Studio.
9. Verify live pass/fail evaluation.
10. Verify nonzero scores block release.
11. Verify untested checks warn but may continue.
12. Verify normal desktop shortcuts and CSSClient/Phocus remain after release.
13. Verify Development Studio and plaintext author files are absent.
