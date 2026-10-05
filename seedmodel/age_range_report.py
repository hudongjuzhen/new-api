"""Review how age_range_rules.py classifies the catalogue (read-only).

Run: C:\\Python313\\python.exe seedmodel\\age_range_report.py [csv]

Writes seedmodel\\age_range_report.txt: the per-bucket distribution, the rows that
fell back to the default bucket, the rows where several buckets matched and the
priority decided, and a sample of every bucket with the evidence used.
"""

import csv
import pathlib
import sys
from collections import Counter

HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

from age_range_rules import AGE_RANGES, classify  # noqa: E402

CSV_PATH = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else HERE / "火山引擎音色库列表.csv"
OUT = HERE / "age_range_report.txt"

rows = list(csv.DictReader(CSV_PATH.open(encoding="utf-8-sig", newline="")))
results = [(record, *classify(record["name"], record["description"])) for record in rows]

lines = [f"source: {CSV_PATH.name}", f"rows: {len(results)}", ""]
distribution = Counter(age_range for _, age_range, _ in results)
for age_range in AGE_RANGES:
    lines.append(f"{age_range:7s} {distribution.get(age_range, 0):3d}")
lines.append("")

lines.append("evidence kind:")
for kind, count in Counter(
    evidence.split(":")[0] for _, _, evidence in results
).most_common():
    lines.append(f"  {kind:10s} {count}")
lines.append("")

lines.append("rows decided by the fallback default (review these):")
for record, age_range, evidence in results:
    if evidence == "fallback":
        lines.append(
            f"  [{age_range}] {record['name']} | {record['gender']} | {record['language']}"
            f" | {record['description'][:44]}"
        )
lines.append("")

lines.append("rows where several buckets matched (priority decided):")
for record, age_range, evidence in results:
    if "also" in evidence:
        lines.append(f"  [{age_range}] {record['name']} | {evidence}")
lines.append("")

lines.append("sample per bucket:")
for age_range in AGE_RANGES:
    lines.append(f"--- {age_range} ---")
    for record, bucket, evidence in [item for item in results if item[1] == age_range][:15]:
        lines.append(f"  {record['name']} | {evidence}")

OUT.write_text("\n".join(lines), encoding="utf-8")
print("wrote", OUT)
