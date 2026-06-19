# will-msg

Parse Microsoft Outlook `.msg` files into structured CSV rows.

The current parser is tuned for the Medford tag emails in `data/`. It reads one `.msg` file, extracts the message body, and emits one CSV row per reported service exception.

## Requirements

- Go 1.23+

## Install dependencies

```bash
go mod tidy
```

## Run the parser

```bash
go run . -input "data/Medford Tags 01_02_26.msg" -output output/medford_010226.csv
```

## Output schema

Each row in the CSV represents one parsed entry from the email body.

| Column          | Meaning                                                 |
| --------------- | ------------------------------------------------------- |
| `source_file`   | Original `.msg` filename                                |
| `subject`       | Outlook subject line                                    |
| `message_date`  | Message timestamp, normalized to RFC3339 when available |
| `from`          | Sender email address                                    |
| `to`            | Raw `To` recipients from the parsed message             |
| `cc`            | Raw `CC` recipients from the parsed message             |
| `reported_at`   | Timestamp line associated with the entry                |
| `dispatcher`    | Dispatcher token from the timestamp line                |
| `entry_index`   | 1-based entry number within the message                 |
| `raw_entry`     | Original parsed service text                            |
| `location_hint` | Best-effort address/location prefix                     |
| `issue_type`    | Best-effort normalized issue classification             |
| `issue_time`    | Trailing on-route time such as `0832AM`, if present     |

## Example

Input body snippet:

```text
01/02/2026 08:36:08 SSAWALLI
114 SOUTH ST RECYC NOT OUT 0832AM
8 AND 14 CURTIS ST RECYC NOT OUT 0831AM
```

Output rows:

```csv
source_file,subject,message_date,from,to,cc,reported_at,dispatcher,entry_index,raw_entry,location_hint,issue_type,issue_time
Medford Tags 01_02_26.msg,Medford Tags 01.02.26,2026-01-02T20:25:55Z,ssawalli@wm.com,...,,2026-01-02T08:36:08-05:00,SSAWALLI,2,114 SOUTH ST RECYC NOT OUT 0832AM,114 SOUTH ST,recyc_not_out,0832AM
Medford Tags 01_02_26.msg,Medford Tags 01.02.26,2026-01-02T20:25:55Z,ssawalli@wm.com,...,,2026-01-02T08:36:08-05:00,SSAWALLI,3,8 AND 14 CURTIS ST RECYC NOT OUT 0831AM,8 AND 14 CURTIS ST,recyc_not_out,0831AM
```

## Verify

Run the tests:

```bash
go test ./...
```

Generate a sample CSV:

```bash
go run . -input "data/Medford Tags 01_02_26.msg" -output output/medford_010226.csv
```

## Current assumptions

- Input is a Microsoft Outlook `.msg` file.
- The body contains alternating timestamp lines and one or more service-entry lines.
- Footer/signature lines are ignored.
- `location_hint` and `issue_type` are heuristics; `raw_entry` is the canonical source text.

## Next extension points

- Parse an entire directory of `.msg` files into one combined CSV.
- Write SQLite in addition to CSV.
- Add stronger address parsing and issue normalization.
- Add a watched `incoming/` folder for daily automation.
