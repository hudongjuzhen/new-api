"""Convert an Excel voice library into a Voice Plaza import CSV.

Run: C:\\Python313\\python.exe seedmodel\\build_voice_csv.py [source.xlsx] [out.csv]

The output uses exactly the columns the plugin's CSV import documents
(docs/zsy-voiceplaza-api.md §3.2), so the file can be imported either through
POST /dashboard/zsy/voice/import or by the _scripts/voiceplaza-seed-volc seeder.

Mapping and clean-up rules for the Volcengine sheet
(场景 | 音色名称 | 简介 | 音色ID | 头像URL | 音频示例URL):

  场景          -> scenes      (split on separators, the spreadsheet's "#N/A" dropped)
  音色名称      -> name        (falls back to the voice id when the cell is empty)
  简介          -> description
  音色ID        -> voice_type
  头像URL       -> avatar_url
  音频示例URL   -> audio_url
  derived       -> gender, language (both encoded in the voice id)
  fixed         -> enabled=true, sort_order=row order
"""

import csv
import pathlib
import re
import sys

import openpyxl

HERE = pathlib.Path(__file__).resolve().parent
SOURCE = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else HERE / "火山引擎音色库列表.xlsx"
OUT = pathlib.Path(sys.argv[2]) if len(sys.argv) > 2 else HERE / "火山引擎音色库列表.csv"

# Languages Volcengine encodes in the voice id; anything outside the set is not
# treated as a language token.
LANGUAGES = {
    "zh", "en", "ja", "ko", "th", "id", "vi", "pt", "mx", "es", "ru", "fr",
    "de", "it", "ar", "tl", "ms", "nl", "pl", "tr", "hi", "sv", "da", "fi",
    "no", "cs",
}
GENDERS = {"female", "male"}

HEADER = [
    "name",
    "description",
    "voice_type",
    "gender",
    "age_range",
    "language",
    "scenes",
    "avatar_url",
    "audio_url",
    "audio_name",
    "audio_size",
    "enabled",
    "sort_order",
]


def scenes_of(raw):
    parts = [part.strip() for part in re.split(r"[,，、;；|]", raw)]
    return [part for part in parts if part and part != "#N/A"]


def token_of(voice_id, vocabulary):
    for token in voice_id.split("_"):
        if token in vocabulary:
            return token
    return ""


def main():
    book = openpyxl.load_workbook(SOURCE, read_only=True, data_only=True)
    sheet = book.worksheets[0]
    rows = []
    for index, row in enumerate(sheet.iter_rows(values_only=True)):
        if index == 0:
            continue  # header
        cells = ["" if cell is None else str(cell).strip() for cell in row]
        if any(cells):
            rows.append(cells)

    seen_names = set()
    name_fallbacks = 0
    records = []
    for index, cells in enumerate(rows, start=1):
        scene, name, description, voice_id, avatar, audio = (cells + [""] * 6)[:6]
        if name == "":
            # A row that carries only an id is still usable; the id keeps the
            # name unique, which the unique index requires.
            name = voice_id
            name_fallbacks += 1
        if name in seen_names:
            name = f"{name} ({voice_id})"
        seen_names.add(name)

        records.append(
            {
                "name": name,
                "description": description,
                "voice_type": voice_id,
                "gender": token_of(voice_id, GENDERS),
                "age_range": "",
                "language": token_of(voice_id, LANGUAGES),
                "scenes": ",".join(scenes_of(scene)),
                "avatar_url": avatar,
                "audio_url": audio,
                "audio_name": "",
                "audio_size": "",
                "enabled": "true",
                "sort_order": str(index),
            }
        )

    with OUT.open("w", encoding="utf-8-sig", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=HEADER)
        writer.writeheader()
        writer.writerows(records)

    languages = sorted({record["language"] for record in records if record["language"]})
    print(f"wrote {OUT} rows={len(records)} nameFallbacks={name_fallbacks}")
    print(f"languages ({len(languages)}): {', '.join(languages)}")


main()
