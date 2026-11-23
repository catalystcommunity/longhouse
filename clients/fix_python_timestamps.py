"""Keep the JSON helpers for csilgen 0.2.15 compatible with native timestamps."""

import re
import sys
from pathlib import Path


def fix_record(record):
    fields = re.findall(r"^    (\w+): ([^\n=]*\bdatetime\b[^\n=]*)", record, re.M)
    for name, annotation in fields:
        source = f"self.{name}"
        value = f"data['{name}']"
        encode = f"_timestamp_text({source})"
        decode = f"_parse_timestamp({value})"
        if "List[" in annotation:
            encode = f"[_timestamp_text(item) for item in {source}]"
            decode = f"[_parse_timestamp(item) for item in {value}]"
        record = record.replace(f"result['{name}'] = {source}\n", f"result['{name}'] = {encode}\n")
        record = record.replace(
            f"{name}=data.get('{name}')",
            f"{name}=({decode} if data.get('{name}') is not None else None)",
        )
    return record


path = Path(sys.argv[1])
preamble, *records = path.read_text().split("\n@dataclass\n")
preamble = preamble.replace("from datetime import datetime\n", "from datetime import datetime, timezone\n")
preamble += '''

def _timestamp_text(value: datetime) -> str:
    if value.utcoffset() is None:
        raise ValueError("timestamp requires a UTC offset")
    return value.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")


def _parse_timestamp(value: str) -> datetime:
    instant = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if instant.utcoffset() is None:
        raise ValueError("timestamp requires a UTC offset")
    return instant.astimezone(timezone.utc)

'''
path.write_text("\n@dataclass\n".join([preamble, *map(fix_record, records)]))
