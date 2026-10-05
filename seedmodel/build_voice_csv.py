"""Convert an Excel voice library into a Voice Plaza import CSV.

Run: C:\\Python313\\python.exe seedmodel\\build_voice_csv.py [source.xlsx] [out.csv]

The output uses exactly the columns the plugin's CSV import documents
(docs/zsy-voiceplaza-api.md §3.2), so the file can be imported either through
POST /dashboard/zsy/voice/import or by the _scripts/voiceplaza-seed-volc seeder.

Mapping and clean-up rules for the Volcengine sheet
(场景 | 音色名称 | 简介 | 音色ID | 头像URL | 音频示例URL):

  场景          -> scenes      (split on separators; the spreadsheet's "#N/A" dropped)
  音色名称      -> name        (falls back to the voice id when the cell is empty)
  简介          -> description (U+FFFD garbage removed, see below)
  音色ID        -> voice_type
  头像URL       -> avatar_url
  音频示例URL   -> audio_url
  derived       -> gender, language (encoded in the voice id)
  derived       -> age_range (see age_range_rules.py; the sheet has no such column)
  fixed         -> enabled=true, sort_order=row order

Source-file repairs, all reported in the summary:

  * 44 cells carry U+FFFD (the workbook was exported with broken characters);
    those characters are stripped.
  * 30 rows lost their scene value to "#N/A" while the scene name leaked into the
    end of the introduction ("…魅力十足。有声阅读"); the trailing scene name is
    moved back into `scenes` and removed from the introduction.
"""

import csv
import pathlib
import re
import sys
from collections import Counter

import openpyxl

HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

from age_range_rules import AGE_RANGES, classify  # noqa: E402

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

# Scene names that appear both in the 场景 column and, for the rows whose scene
# cell is "#N/A", at the end of the introduction.
SCENE_NAMES = [
    "通用场景",
    "角色扮演",
    "视频配音",
    "客服场景",
    "有声阅读",
    "教育场景",
    "外语音色",
    "教学场景",
    "趣味口音",
    "动漫",
    "娱乐",
]

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


def split_trailing_scene(description):
    """Return (description without the leaked scene name, that scene name or '').

    The source workbook appends the 场景 value to the end of most introductions
    ("…亲和力十足。有声阅读"), which reads as noise in the plaza and duplicates the
    scene tag. The trailing name is cut off and returned so the caller can make
    sure the scene list still carries it.
    """
    stripped = description.replace("\ufffd", "").strip()
    for scene in SCENE_NAMES:
        if stripped.endswith(scene):
            head = stripped[: -len(scene)]
            # A separator left dangling by the cut ("……女生，动漫") is dropped.
            head = head.rstrip(" \t，,、；;")
            return head, scene
    return stripped, ""


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
    repaired_junk = 0
    repaired_scenes = 0
    records = []
    ages = Counter()
    for index, cells in enumerate(rows, start=1):
        scene, name, description, voice_id, avatar, audio = (cells + [""] * 6)[:6]

        if "\ufffd" in description:
            repaired_junk += 1
        description, leaked_scene = split_trailing_scene(description)

        scenes = scenes_of(scene)
        if leaked_scene and leaked_scene not in scenes:
            scenes.append(leaked_scene)
            repaired_scenes += 1

        if name == "":
            # A row that carries only an id is still usable; the id keeps the
            # name unique, which the unique index requires.
            name = voice_id
            name_fallbacks += 1
        if name in seen_names:
            name = f"{name} ({voice_id})"
        seen_names.add(name)

        age_range, _evidence = classify(name, description)
        ages[age_range] += 1

        records.append(
            {
                "name": name,
                "description": description,
                "voice_type": voice_id,
                "gender": token_of(voice_id, GENDERS),
                "age_range": age_range,
                "language": token_of(voice_id, LANGUAGES),
                "scenes": ",".join(scenes),
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

    languages = sorted({r["language"] for r in records if r["language"]})
    print(f"wrote {OUT} rows={len(records)} nameFallbacks={name_fallbacks}")
    print(f"repaired: U+FFFD cells={repaired_junk} scenes recovered from remarks={repaired_scenes}")
    print("age ranges: " + ", ".join(f"{age}={ages.get(age, 0)}" for age in AGE_RANGES))
    print(f"languages ({len(languages)}): {', '.join(languages)}")


main()
