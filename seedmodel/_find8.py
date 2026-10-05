"""Search both databases for the eight hand-typed voices (read-only).

Run: C:\\Python313\\python.exe seedmodel\\_find8.py
Writes seedmodel\\_find8.txt (UTF-8).
"""

import pathlib
import re
import sqlite3

import pymysql

ROOT = pathlib.Path(__file__).resolve().parent.parent
OUT = pathlib.Path(__file__).with_name("_find8.txt")
NEEDLES = ["尖细嗓", "冷峻男音", "尖细", "冷峻"]

lines = []

# ── deployed MySQL ──────────────────────────────────────────────────────────
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
with connection:
    with connection.cursor() as cursor:
        cursor.execute("SELECT COUNT(*) AS n FROM zsy_voices")
        lines.append(f"deployed zsy_voices rows: {cursor.fetchone()['n']}")
        cursor.execute("SELECT id, name, voice_type, created_at, updated_at FROM zsy_voices")
        rows = cursor.fetchall()
        for needle in NEEDLES:
            hits = [row for row in rows if needle in (row["name"] or "")]
            lines.append(f"  contains {needle!r}: {len(hits)}")
            for row in hits:
                lines.append(f"    {row}")
        lines.append("  newest 10 rows by id (what a fresh page load would show first):")
        for row in sorted(rows, key=lambda r: r["id"])[-10:]:
            lines.append(f"    id={row['id']} name={row['name']!r} created={row['created_at']}")
        lines.append("  rows sorted the way the API returns them (sort_order, id):")
        cursor.execute(
            "SELECT id, name, sort_order FROM zsy_voices ORDER BY sort_order, id LIMIT 10"
        )
        for row in cursor.fetchall():
            lines.append(f"    sort={row['sort_order']} id={row['id']} name={row['name']!r}")

# ── local sqlite ────────────────────────────────────────────────────────────
db_path = ROOT / "one-api.db"
if db_path.exists():
    sq = sqlite3.connect(f"file:{db_path}?mode=ro", uri=True)
    tables = [
        row[0]
        for row in sq.execute("SELECT name FROM sqlite_master WHERE type='table'")
    ]
    lines.append("")
    lines.append(f"local {db_path.name}: {len(tables)} tables, voice tables:")
    voice_tables = [t for t in tables if "voice" in t.lower()]
    lines.append(f"  {voice_tables}")
    sq.close()

OUT.write_text("\n".join(lines), encoding="utf-8")
print("wrote", OUT)
