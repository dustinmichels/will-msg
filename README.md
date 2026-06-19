# will-msg

Parse Microsoft Outlook `.msg` files into structured CSV rows.

The parser is tuned for the Medford tag emails in `data/`. It accepts either one `.msg` file or a directory containing `.msg` files, then emits one CSV row per reported service exception.

## Requirements

- Go 1.23+

## Install dependencies

```bash
go mod tidy
```

## Run the parser for one file

```bash
go run . -input "data/Medford Tags 01_02_26.msg" -output output/medford_010226.csv
```

## Run the parser for a directory of `.msg` files

```bash
go run . -input data -output output/all_data.csv
```

Directory mode reads every `*.msg` file in that directory and combines all parsed rows into one CSV. If one file is malformed or has no parseable rows, it is skipped with a warning and the rest continue.

## Optional batch wrapper

If you want a short command, the repo also includes:

```bash
./scripts/run_all.sh
```

Custom input/output paths:

```bash
./scripts/run_all.sh data output/all_data.csv
```

That script is just a thin wrapper around the directory-mode CLI.

## Output schema

Each row in the CSV represents one parsed entry from the email body.

| Column | Meaning |
| --- | --- |
| `source_file` | Original `.msg` filename |
| `subject` | Outlook subject line |
| `message_date` | Message timestamp, normalized to RFC3339 when available |
| `reported_at` | Timestamp line associated with the entry |
| `dispatcher` | Dispatcher token from the timestamp line |
| `row_in_message` | 1-based row order within the source message |
| `raw_entry` | Original parsed service text |
| `location_hint` | Best-effort address/location prefix |
| `issue_type` | Best-effort normalized issue classification |
| `issue_time` | Trailing on-route time such as `0832AM`, if present |

## Example

Input body snippet:

```text
01/02/2026 08:36:08 SSAWALLI
114 SOUTH ST RECYC NOT OUT 0832AM
8 AND 14 CURTIS ST RECYC NOT OUT 0831AM
```

Output rows:

```csv
source_file,subject,message_date,reported_at,dispatcher,row_in_message,raw_entry,location_hint,issue_type,issue_time
Medford Tags 01_02_26.msg,Medford Tags 01.02.26,2026-01-02T20:25:55Z,2026-01-02T08:36:08-05:00,SSAWALLI,2,114 SOUTH ST RECYC NOT OUT 0832AM,114 SOUTH ST,recyc_not_out,0832AM
Medford Tags 01_02_26.msg,Medford Tags 01.02.26,2026-01-02T20:25:55Z,2026-01-02T08:36:08-05:00,SSAWALLI,3,8 AND 14 CURTIS ST RECYC NOT OUT 0831AM,8 AND 14 CURTIS ST,recyc_not_out,0831AM
```

## Verify

Run the tests:

```bash
go test ./...
```

Generate one sample CSV:

```bash
go run . -input "data/Medford Tags 01_02_26.msg" -output output/medford_010226.csv
```

Generate one combined CSV for all sample files:

```bash
go run . -input data -output output/all_data.csv
```

## Current assumptions

- Input is a Microsoft Outlook `.msg` file.
- Directory mode only reads `*.msg` files in the top level of the given directory.
- The body contains alternating timestamp lines and one or more service-entry lines.
- Footer/signature lines are ignored.
- `location_hint` and `issue_type` are heuristics; `raw_entry` is the canonical source text.

## Next extension points

- Write SQLite in addition to CSV.
- Add stronger address parsing and issue normalization.
- Add a watched `incoming/` folder for daily automation.
