# GReal (Go) v1.3.0

Email account enumeration tool (O365 / GMAIL / GSUITE) — Go reimplementation.

You pick a **module** (the target platform) and a **technique** (how to probe):

```
./greal <module> --<technique> -d <domain> [args]
```

Modules (platform):

| Module | What it does |
|--------|--------------|
| `gsuite`  | Google Workspace — SMTP against the auto-detected Google MX |
| `o365`    | Microsoft 365 — web enumeration, or SMTP against the auto-detected O365 MX |
| `generic` | any mail server — SMTP, prompts for the MX server |

Techniques (flag):

| Flag | Transport | Description |
|------|-----------|-------------|
| `--rcpt` | SMTP | RCPT TO recipient enumeration (generic, most reliable) |
| `--vrfy` | SMTP | VRFY user verification (often disabled on modern servers) |
| `--expn` | SMTP | EXPN mailing-list / alias expansion (`--extn` alias) |
| `--web`  | Web  | GetCredentialType enumeration — **`o365` module only** |

`--web` works only with `o365` (Microsoft's public `GetCredentialType` endpoint).
Reliable unauthenticated web enumeration for Google Workspace is no longer
available, so for `gsuite` use `--rcpt`.

Add `-v` / `--verbose` to print every probe result (status/code), not just hits.

> Offensive testing against client assets requires written authorization and a
> defined scope. Web modes send candidate addresses to Microsoft/Google.

> Use only against assets you are explicitly authorized to test. Any intrusion
> test or scan against client assets requires written authorization and a
> defined scope.

## Build

```
go build -o greal .
```


## Usage

```
Usage:
  ./greal <module> --<technique> -d <domain> [args]
  module: gsuite | o365 | generic      technique: --rcpt | --vrfy | --expn | --web
Args
	-d, --domain              set target domain (required)
	-f, --firstname-list      set firstname list file
	-l, --lastname-list       set lastname list file
	-t, --threads             set threads
	    --format              set what kind of email format use for enumeration
		                      [ 1 - Firstname.Lastname ]
		                      [ 2 - FirstnameLastname ]
		                      [ 3 - F.Lastname ]
		                      [ 4 - FLastname ]
		                      [ 5 - firstname ]

	-k, --mail-rate           Ignore 'OK' response and show only email with 'receiving mail at a rate ' status response
	-o, --output              write found accounts to a file (.json / .csv / .txt by extension, labelled CONFIDENCIAL)
	    --max-per-conn        recycle each SMTP connection after N probes (0 = reuse the connection for the whole run)
	-v, --verbose             print every probe result (status/code), not just hits
	-h, --help                show this message
Examples:
	Google Workspace, RCPT, format [ Firstname.Lastname ]
	use: ./greal gsuite --rcpt -d domain.com --format 1 -f wordlist/firstname.txt -l wordlist/lastname.txt
	Microsoft 365, web enumeration, with output file
	use: ./greal o365 --web -d domain.com --format 1 -o results.json
	Microsoft 365, SMTP RCPT, verbose
	use: ./greal o365 --rcpt -d domain.com --format 4 -l wordlist/lastname.txt --verbose
```

## Performance

Each worker keeps one SMTP connection open and runs many `RCPT TO` probes over it
(the connection is reset between recipients). Use `--max-per-conn N` to recycle a
connection every N probes if the MX server rate-limits per connection.

