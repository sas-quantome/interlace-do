###### September 20, 2026: interlace-do internal readme v2

# Internal

```text
├── README.md
├── app
│   ├── README.md
│   ├── a32001
│   │   ├── a32001.go
│   │   ├── a32001_configure.go
│   │   ├── a32001_metadata.go
│   │   └── a32001_synonym.go
│   ├── a32002
│   │   ├── a32002.go
│   │   ├── a32002_args.go
│   │   ├── a32002_reader.go
│   │   └── a32002_writer.go
│   ├── a32003
│   │   ├── a32003.go
│   │   └── a32003_dna_sequence.go
│   └── a32004
│       ├── a32004.go
│       ├── a32004_args.go
│       ├── a32004_finder.go
│       ├── a32004_nucleotide.go
│       ├── a32004_reader.go
│       └── a32004_writer.go
└── pkg
    ├── README.md
    ├── arranger
    │   ├── definer.go
    │   ├── formatter.go
    │   └── formatter_test.go
    └── seeker
        ├── finder.go
        ├── finder_test.go
        └── matcher.go
```

## Contents

### [app](./app)

Application non-shared Go code.

### [pkg](./pkg)

Internal package shared Go code.
