<p align="center"><img src="icon.svg" /></p>

# $$\textcolor{red}{\text{J}}\textcolor{blue}{\text{u}}\textcolor{red}{\text{s}}\textcolor{blue}{\text{tificati}}\textcolor{red}{\text{on}}$$

Justification is a proof-of-concept REST-based JSON validator, with schemas based on the [JSON Schema](https://json-schema.org/) vocabulary.

The REST server contains two endpoints as follows:

|  Endpoint                |  Description  |
|--------------------------|---------------|
| `POST /schema/$SCHEMAID`   | Used to upload valid JSON schemas to the server. |
| `GET  /schema/$SCHEMAID`   | Used to retrieve stores JSON schemas from the server. |
| `POST /validate/$SCHEMAID` | Used to validate JSON against a store schema.<br>Null objects entries will be stripped out of the JSON before validation. |

$SCHEMAID can be any combination of alphanumerical characters as well as underscore '\_' (ascii 95/0x5f) and hypen '-' (ascii 45/0x2d).

## Installation

The following command can be used to install this program:

```
go install vimagination.zapto.org/justification@v1.0.0
```

This will download and compile a `justification` binary into either your $GOPATH/bin or $GOBIN directory.

NB: You will need to have the [Go Programming Language](https://go.dev/) installed in order to use the above command.

## Running

The following are the parameters accepted by this program:

|  Flag  |  Default              |  Description  |
|--------|-----------------------|---------------|
| p      | 8080                  | The port on which the justification binary will serve. |
| d      | $config/justification | The directory where schemas will be store and loaded from. |

NB: $config refers to the output of [os.UserConfigDir](https://pkg.go.dev/os#UserConfigDir).

For example, to run `justification` on port `1234` and using a temporary directory, you could use the following:

```
justification -p 1234 -d /tmp/justification-tmp-dir
```

The server can be stopped by using Ctrl+C (SIGINT).

## Tests

The examples directory contains two files, config-schema.json and config.json.

With `justification` running on its default port, the following command will upload the example schema to the server:

```
curl http://localhost:8080/schema/config-schema -X POST --data-binary @config-schema.json
```

...the following can be used to retrieve the saved schema:

```
curl http://localhost:8080/schema/config-schema
```

...and the following can be used to check the example JSON file against that saved schema:
```
curl http://localhost:8080/validate/config-schema -X POST -d @config.json
```
