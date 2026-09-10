###### September 10, 2026: interlace-do configs readme v1

# Configs

```text
configs
├── README.md
└── input
    ├── uri.txt
    └── url.txt
```

## Contents

### [URI]

A Uniform Resource Identifier (URI) is a unique sequence of characters that identifies an abstract or physical resource, such as a webpage, a full-path filename, an email address, a phone number, a book, a research article, a real-world object, or a concept. A URI list is necessary to ensure that input sources can be uniquely and accurately identified.

- Use this URI file to facilitate applications finding their correct resources.
- There is no need to edit this file unless a URL has changed, **interlace-do** [applications](../cmd/README.md) parse the URI file and select the target resource.
- Before using it verify if the source databases have newer releases or URL updates.

### [URL]

A Uniform Resource Locator (URL) is a specific type of URI that identifies how to access or locate a web resource, usually via a specific protocol, e.g., HTTP, HTTPS, FTP, etc.

- This file contains URLs in the URI file.
- The Bash [scripts](../scripts/README.md) require this file to download all the input files in advance.
- The scripts also check for broken URLs.

[URI]: input/uri.txt
[URL]: input/url.txt