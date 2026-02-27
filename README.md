# GitHub Extension "Go Do"

<!-- cspell:ignore agentic Coilot devx fantazie winget choco wrapup Hmmmm -->
<!-- markdownlint-disable MD033 -->

## The Complete DevX, git-fu Ninja 🥷🏻 and Agentic AI 🤖 Tutorial

**`gh_godo` is a sample GitHub Command Line extension written in Go. It really can't do much. The point is that it will be able to do what ever you fantazie it should do. This tutorial will show you how to setup a repository with _everything you need_ to start passing spec on to an agentic AI bot, and enable you to have a collaborative spec-driven workflow, where the bot does the heavy lifting and you stay in control**

## Topics covered in this tutorial

- [x] Prepare your PC to run Docker and VS Code
- [x] Setup dev container
- [x] Setup basic – always on – linters
- [x] Setup git
- [x] Setup [TakT](https://github.com/devx-cafe/gh-tt)and GitHub workflows to support a Trunk-Based, Pull-Request free workflow
- [x] Setup Copilot MCP server, extension, workflows and RAG-files
- [x] Setup a flow that makes Copilot's Pull-Request centric work process compliant with your trunk-based PR-free flow
- [x] Add a bare-minimum go cmd line project that is compliant with GitHub's `gh` CLI extensions
- [x] Optimize your flow with home-made parameter driven reusable actions

## 🤷‍♂️ How to Get Started

### Use this repo as a template

From the `Code` tab of this repo - In the upper right corner click use template:

<img width="209" height="125" alt="Image" src="https://github.com/user-attachments/assets/fb19f49e-db7b-467f-935c-90ed4f57f0cb" />

### Get ALL branches

<img width="723" height="153" alt="Image" src="https://github.com/user-attachments/assets/2b4e24a0-1f56-4628-9947-63d2839453c1" />

> [!CAUTION]
> Be sure to include _all_ branches
> This is not the default, you will have to enable it.
> If you dont' ...you will have to start over!

### Use the same name as _this_ repo

<img width="719" height="90" alt="Image" src="https://github.com/user-attachments/assets/5f3b282a-b517-4668-95e6-47dfa5b01e11" />

Name the repo `gh-godo` like this one...

## ⚠️ Prerequisites [IMPORTANT]

### 1. Setup your PC to support Docker and VS Code

> [!IMPORTANT]
> You need your PC (host) to support
>
> - VS Code
> - Docker (Docker Desktop)

## 🛠️ D.I.Y

### Resources

#### Docker

- [Mac](https://docs.docker.com/desktop/setup/install/mac-install/)
- [Windows](https://docs.docker.com/desktop/setup/install/windows-install/) (**IMPORTANT**: Use `WSL` as opposed to `Hyper-V`)

#### VS Code

- [Downloads](https://code.visualstudio.com/download)

<details><summary><b>👀 HOW TO DO THIS ON WINDOWS</b></summary>

Using **winget**, the installation process is very straightforward. You can run these commands in a standard PowerShell terminal (or run as Admin). If you do not run as admin Windows may prompt you for permission during the install.

## Setup Windows

### The PowerShell Script

Copy and paste this into your PowerShell window:

```powershell
# 1. Update the winget index
winget update

# 2. Install VS Code
winget install -e --id Microsoft.VisualStudioCode

# 3. Install Docker Desktop
winget install -e --id Docker.DockerDesktop

# 4. Install WSL (This - some times - installs Ubuntu by default, but not always)
# Note: If WSL is already installed, this will simply skip or update it.
wsl --install

# 5. Install Ubuntu (WSL - sometimes - installs Ubuntu by default, but not always)
# Note: No harm in running it again.
winget install -e --id Canonical.Ubuntu.2404

```

After running the script, you'll need to do a few manual "handshakes" and additional processes.

The first thing your need to do is to

- [ ] **Restart your PC**

**This is required** for the WSL and Docker features to fully register in the Windows kernel.

## Setup Ubuntu

Open the "Ubuntu" app from your Start menu. First time it will ask you to create

- [ ] user
- [ ] password

### Install WSL utilities

_While still in your Ubuntu terminal..._

Ubuntu in WSL is headless and out-of-the-box it doesn't know how to start a browser from the terminal. We need that:

```bash
sudo apt update \
  && sudo apt install wslu -y \
  && echo 'export BROWSER=wslview' >> ~/.bashrc \
  && source ~/.bashrc
```

### Initialize git

_While still in your Ubuntu terminal..._

- [ ] Verify git is installed by running `git --version` (if it's not run `sudo apt update && sudo apt install git`)
- [ ] Configure git with `git config --global user.name "<Your Name>"`
- [ ] Configure git with `git config --global user.name "<you@email.com>"`

### Setup GitHub CLI

We also want to be able to use the GitHub Cli.

- [ ] Run `sudo apt install gh` (Wait! - You've got options, see below)

> [!CAUTION]
> To install the GitHub CLI (gh) on Ubuntu, it is best to use the official GitHub repository rather than the default Ubuntu one. The version in the standard Ubuntu "apt" library is often outdated, which can cause issues with authentication.

**Use Ubuntu repository:**

```bash
sudo apt update \
&& sudo apt install gh -y
```

**Use GitHub repository**

```bash
(type -p wget >/dev/null || (sudo apt update && sudo apt-get install wget -y)) \
	&& sudo mkdir -p -m 755 /etc/apt/keyrings \
	&& wget -qO- https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null \
	&& sudo chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
	&& echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
	&& sudo apt update \
	&& sudo apt install gh -y
```

> [!NOTE]
> At time of this writing (February 2026) the first approach will give you `gh` in version 2.45.0 and the second will give you version 2.87.3.

### Setup GitHub authentication

```bash
gh auth login --scopes project --web --hostname github.com --git-protocol https --clipboard
```

The one-time code is already copied to the clipboard, so just press enter, paste the clipboard into the browser, accept the app, close the browser again.

> [!NOTE]
> The `gh` cli looks for authentication in the following places in the following order, and stops with the first it finds:
>
> - `$GH_TOKEN`
> - `$GITHUB_TOKEN`
> - `~/.config/gh/hosts.yml`

Your token is now stored in your `/.config/gh/hosts.yml` so it's persisted between logins.

But for easier integration with Dev containers we want this to be store in an environment variable:

```bash
echo "export GH_TOKEN=$(gh auth token)" >> ~/.profile
source ~/.profile
```

If you run `gh auth status` you will see that you now have both.

- **Exit Ubuntu** (run `exit`)
- **Reopen Ubuntu** and rerun `gh auth status` to see that both auth sources settings are now persisted.

#### Enable Docker Integration

- Open **Docker Desktop** on Windows.
- Go to **Settings (⚙ icon)**.

- [ ] **> Resources > WSL Integration** Ensure _"Enable integration with my default WSL distro"_ is enabled.
- [ ] **> Resources > Advanced** If you find that Docker is slow to wake up when you run your first command, you can experiment with disable _"Resource Saver"_
- [ ] **> General** Make sure _"Use containerd for pulling and storing images"_ is checked. It allows for faster image pulls and better support for multi-platform builds.

> [!TIP]
> **The `.wslconfig` file**
> WSL can sometimes try to "eat" all your RAM. You can cap its appetite by creating a configuration file in Windows.
>
> - Open PowerShell and type: `notepad "$env:USERPROFILE\.wslconfig"`
>   Paste this in (adjusting memory to about 50% of your total RAM):
>
> ```ini
> Ini, TOML
> [wsl2]
> # Limit VM memory to 8GB (adjust to your needs)
> memory=8GB
> # Give it 4 cores
> processors=4
> # Automatically reclaim unused memory from the VM back to Windows
> autoMemoryReclaim=gradual
> ```
>
> Save and restart WSL by running `wsl --shutdown` in PowerShell.

#### Install the VS Code MCP servers and Extensions

- Open VS Code.

Enable MCP servers marketplace

- [ ] Open the extensions panel (CTRL+SHIFT+X) search for `@mcp`, click _"Enable MCP servers marketplace"_
- [ ] Find the **"GitHub"** MCP server and install it. (It will take you to the webbrowser to authorize the app)

Find and install (or just verify) the following extensions.

- [ ] **WSL** _(Microsoft)_
- [ ] **Remote Development** _(Microsoft)_
- [ ] **Dev Containers** _(Microsoft)_
- [ ] **GitHub Copilot Chat** _(GitHub)_

Hmmmm - That's it for now let's see everything in play

- **Exit (quit) VS Code**
- **Exit Ubuntu terminal** (run `exit`)
- **Reopen Ubuntu terminal** (run `gh auth status` to test the your token is persisted as `$GH_TOKEN` too)

👇 Now run the final test👇

</details>

## 🧪 The final test

Let's clone a simple, open source repo, that uses dev containers and see if we can make it work:

```bash
mkdir -p github/devx-cafe
cd github/devx-cafe
gh repo clone devx-cafe/takt-actions
code takt-actions
```

When the project opens in VS Code you will be asked if you want to "Reopen in Dev Container" - choose that option.

Docker will now build the container.
