###### October 22, 2026: interlace-do readme v2

# interlace-do

Welcome to **interlace-do**! Interlace Go toolset and usage with the **interlace-ex** Gene data playground.

![interlace-do](assets/interlace-do.png)

## Contents

1) [Download and Install Go](#1-download-and-install-go)
2) [Declare Your Interlace Codebase Path](#2-declare-your-interlace-codebase-path)
3) [Create Your Interlace Codebase Directory](#3-create-your-interlace-codebase-directory)
4) [Clone the interlace-do Repository](#4-clone-the-interlace-do-repository)
5) [Check the Makefile](#5-check-the-makefile)
6) [Build the Toolchain](#6-build-the-toolchain)
7) [Make Install](#7-make-install)
8) [Cleanup](#8-cleanup)

### 1. Download and Install Go

Download and install Go from [https://go.dev/doc/install](https://go.dev/doc/install). It takes less than 5 minutes to complete!

### 2. Declare Your Interlace Codebase Path
Declare a place where to clone the GitHub repositories the standard way somewhere in your computer. Include the `sas-quantome` directory to store multiple Interlace repositories:
```bash
REPO_PATH="${HOME}/github.com/sas-quantome"
```

### 3. Create Your Interlace Codebase Directory
Create the directory:
```bash
mkdir -p "${REPO_PATH}"
```

### 4. Clone the interlace-do Repository
Clone the repository in the `sas-quantome` folder:
```bash
cd "${REPO_PATH}"
git clone https://github.com/sas-quantome/interlace-do.git
```

### 5. Check the Makefile
Review the Makefile and then run the following commands:
```bash
cd "${REPO_PATH}/interlace-do"
make help
```

### 6. Build the Toolchain
If all is well in your installation, you will find the compiled products in the `bin` folder:
```bash
cd "${REPO_PATH}/interlace-do"
make build
```

### 7. Make Install
Compile the software and install the binaries, you will find them in your `$GOPATH/bin`:
```bash
cd "${REPO_PATH}/interlace-do"
make install
```

### 8. Cleanup
Remove the `bin` folder:
```bash
cd "${REPO_PATH}/interlace-do"
make clean
```

***
*&copy; 2026, Société par Actions Simplifiée Quantome, Paris 75014, France. All rights reserved.*
