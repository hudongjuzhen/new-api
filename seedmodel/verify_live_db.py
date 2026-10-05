"""Verify the seeded Voice Plaza rows on the deployed MySQL (read-only).

Run: C:\\Python313\\python.exe seedmodel\\verify_live_db.py
Writes seedmodel\\_live_verify.txt (UTF-8).
"""

import pathlib
import re
from collections import Counter

import pymysql

ROOT = pathlib.Path(__file__).resolve().parent.parent
OUT = pathlib.Path(__file__).with_name("_live_verify.txt")

dsn = ""
for line in (ROOT / ".env").read_text(encoding="utf-8").splitlines():
    if line.startswith("SQL_DSN="):
        dsn = line.split("=", 1)[1].strip()
        break

parts = re.match(
    r"(?P<user>[^:]+):(?P<password>.*)@tcp\((?P<host>[^:]+):(?P<port>\d+)\)/(?P<db>[^?]+)",
    dsn,
).groupdict()

connection = pymysql.connect(
    host=parts["host"],
    port=int(parts["port"]),
    user=parts["user"],
    password=parts["password"],
    database=parts["db"],
    connect_timeout=10,
    charset="utf8mb4",
    cursorclass=pymysql.cursors.DictCursor,
)

lines = []
with connection:
    with connection.cursor() as cursor:
        cursor.execute("SHOW COLUMNS FROM zsy_voices")
        lines.append("columns: " + ", ".join(c["Field"] for c in cursor.fetchall()))

        cursor.execute(
            "SELECT name, description, voice_type, gender, age_range, language, scenes,"
            " avatar_url, audio_url, enabled, sort_order FROM zsy_voices ORDER BY sort_order, id"
        )
        rows = cursor.fetchall()
        lines.append(f"rows: {len(rows)}")
        lines.append(f"distinct voice_type: {len({r['voice_type'] for r in rows})}")
        lines.append(f"distinct name: {len({r['name'] for r in rows})}")
        lines.append(f"age_range: {dict(Counter(r['age_range'] for r in rows))}")
        lines.append(f"gender: {dict(Counter(r['gender'] for r in rows))}")
        lines.append(f"language: {dict(Counter(r['language'] for r in rows).most_common())}")
        lines.append(f"scenes: {dict(Counter(r['scenes'] for r in rows).most_common(6))}")
        lines.append(f"enabled: {dict(Counter(r['enabled'] for r in rows))}")
        lines.append(f"avatar empty: {sum(1 for r in rows if not r['avatar_url'])}")
        lines.append(f"audio empty: {sum(1 for r in rows if not r['audio_url'])}")
        lines.append(f"age_range empty: {sum(1 for r in rows if not r['age_range'])}")
        lines.append(
            f"sort_order range: {min(r['sort_order'] for r in rows)}..{max(r['sort_order'] for r in rows)}"
        )
        lines.append("")
        lines.append("age_range x gender:")
        cross = Counter((r["age_range"], r["gender"]) for r in rows)
        for (age_range, gender), count in sorted(cross.items()):
            lines.append(f"  {age_range:7s} {gender:7s} {count}")
        lines.append("")
        lines.append("first 3 rows:")
        for row in rows[:3]:
            lines.append(
                "  "
                + " | ".join(
                    str(row[key])
                    for key in (
                        "name",
                        "voice_type",
                        "gender",
                        "age_range",
                        "language",
                        "scenes",
                        "enabled",
                        "sort_order",
                    )
                )
            )
        lines.append("")
        lines.append("previously existing row (id=1) after the upsert:")
        cursor.execute(
            "SELECT id, name, description, gender, language, scenes, avatar_url, audio_url"
            " FROM zsy_voices WHERE id = 1"
        )
        for row in cursor.fetchall():
            for key, value in row.items():
                lines.append(f"  {key}: {value}")

OUT.write_text("\n".join(lines), encoding="utf-8")
print("wrote", OUT)
