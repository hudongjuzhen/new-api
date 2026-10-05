"""Search every text column of every table in the deployed DB for the names.

Run: C:\\Python313\\python.exe seedmodel\\_search_all.py
Writes seedmodel\\_search_all.txt (UTF-8). Read-only.
"""

import pathlib
import re

import pymysql

ROOT = pathlib.Path(__file__).resolve().parent.parent
OUT = pathlib.Path(__file__).with_name("_search_all.txt")
NEEDLES = ["尖细嗓", "冷峻男音"]

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
        cursor.execute(
            "SELECT table_name, column_name, data_type FROM information_schema.columns"
            " WHERE table_schema = DATABASE()"
            " AND data_type IN ('varchar','text','mediumtext','longtext','char')"
            " ORDER BY table_name, ordinal_position"
        )
        columns = cursor.fetchall()
        lines.append(f"text columns to search: {len(columns)}")

        for needle in NEEDLES:
            found = 0
            for column in columns:
                table = column["table_name"]
                name = column["column_name"]
                try:
                    cursor.execute(
                        f"SELECT COUNT(*) AS n FROM `{table}` WHERE `{name}` LIKE %s",
                        (f"%{needle}%",),
                    )
                    count = cursor.fetchone()["n"]
                except Exception as error:  # noqa: BLE001 - diagnostic dump
                    lines.append(f"  {table}.{name}: query failed {error}")
                    continue
                if count:
                    found += count
                    lines.append(f"  FOUND {needle!r} in {table}.{name}: {count} rows")
                    cursor.execute(
                        f"SELECT * FROM `{table}` WHERE `{name}` LIKE %s LIMIT 3",
                        (f"%{needle}%",),
                    )
                    for row in cursor.fetchall():
                        lines.append(f"    {row}")
            lines.append(f"{needle!r}: total hits {found}")

OUT.write_text("\n".join(lines), encoding="utf-8")
print("wrote", OUT)
