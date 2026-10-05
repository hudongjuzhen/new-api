"""Age-range classification for the Voice Plaza catalogue.

The plaza stores a five-bucket age range (`child` / `teen` / `young` / `middle` /
`senior`, see docs/zsy-voiceplaza-api.md §4). Provider catalogues such as the
Volcengine workbook do not ship that column, so it is derived here from the two
fields that do describe the voice: its name and its introduction.

How it works
------------
Every bucket owns a keyword list. A row is classified by walking the buckets in
the priority order below and taking the first bucket whose keyword appears in the
name or the introduction:

    senior > child > teen > middle > young

The order encodes the cases where one row carries several signals: an explicit
"童声" is a stronger statement than a soft "甜美", and "中年男老师" is more
specific than the "青年" it also mentions. Rows that match nothing fall back to
`DEFAULT_AGE_RANGE`, which the caller reports so the guessed rows stay visible.

`classify` returns the bucket plus the evidence it used (`name:少年`, `desc:大叔`,
`fallback`), so a report can show exactly why each row landed where it did.
"""

AGE_RANGES = ["child", "teen", "young", "middle", "senior"]

DEFAULT_AGE_RANGE = "young"

# Priority order: the first bucket with a hit wins.
RULES = [
    (
        "senior",
        [
            "爷爷",
            "奶奶",
            "婆婆",
            "外公",
            "外婆",
            "老年",
            "老人",
            "长者",
            "长辈",
            "大爷",
            "老太太",
            "苍老",
            "矍铄",
            "岁月沉淀",
        ],
    ),
    (
        "child",
        [
            "童声",
            "儿童",
            "少儿",
            "幼童",
            "女童",
            "男童",
            "萝莉",
            "萌娃",
            "萌仔",
            "稚嫩",
            "奶气",
            "小朋友",
            "宝宝",
            "婴儿",
            "童趣",
            "天真烂漫",
            "小女孩",
            "小丸子",
            "佩奇",
            "孩童",
        ],
    ),
    (
        "teen",
        [
            "少年",
            "少女",
            "高中生",
            "中学生",
            "初中生",
            "学弟",
            "学妹",
            "校园",
            "青春期",
            "男高",
            "女高",
            "稚气",
        ],
    ),
    (
        "middle",
        [
            "中年",
            "大叔",
            "叔叔",
            "阿姨",
            "妈妈",
            "爸爸",
            "总裁",
            "董事长",
            "上司",
            "老板",
            "职场",
            "成熟",
            "御姐",
            "女强人",
            "长辈音",
        ],
    ),
    (
        "young",
        [
            "青年",
            "大学生",
            "男大",
            "女大",
            "小哥",
            "小妹",
            "妹妹",
            "姐姐",
            "小姐姐",
            "姑娘",
            "年轻",
            "元气",
            "青春",
            "活力",
            "阳光",
            "甜美",
            "甜妹",
            "奶狗",
            "年下",
            "男友",
            "女友",
            "初恋",
            "闺蜜",
            "开朗",
            "活泼",
            "俏皮",
            "灵动",
            "清爽",
            "清新",
        ],
    ),
]

# Second pass for rows with no explicit age word: the tone and the role still say
# something about the voice's age. Mature tone/role words are checked before
# youthful ones, so "傲娇霸总" reads as a mature persona while "温柔女神" stays
# young.
EXTRA_RULES = [
    (
        "middle",
        [
            "叔音",
            "霸总",
            "高僧",
            "译制",
            "浑厚",
            "磁性",
            "沉稳",
            "老成",
            "阅历",
            "资深",
            "大女主",
            "教授",
            "校长",
            "播音",
            "播报",
            "解说",
            "旁白",
            "讲师",
            "医生",
            "律师",
            "职场",
        ],
    ),
    (
        "young",
        [
            "女孩",
            "女生",
            "男生",
            "女神",
            "公主",
            "女王",
            "大小姐",
            "公子",
            "少爷",
            "治愈",
            "温柔",
            "亲切",
            "可爱",
            "软萌",
            "呆萌",
            "清纯",
            "帅气",
            "俊朗",
            "潇洒",
            "幽默",
            "热情",
            "细腻",
            "正能量",
            "邻家",
            "陪伴",
            "恋人",
            "情感",
            "傲娇",
            "病娇",
            "腹黑",
        ],
    ),
]


def classify(name: str, description: str):
    """Return (age_range, evidence) for one catalogue row."""
    sources = (("name", name or ""), ("desc", description or ""))

    matches = []
    for rules in (RULES, EXTRA_RULES):
        for age_range, keywords in rules:
            hit = None
            for label, text in sources:
                for keyword in keywords:
                    if keyword in text:
                        hit = (age_range, f"{label}:{keyword}")
                        break
                if hit is not None:
                    break
            if hit is not None:
                matches.append(hit)

    if not matches:
        return DEFAULT_AGE_RANGE, "fallback"

    winner = matches[0]
    if len({age_range for age_range, _ in matches}) > 1:
        # More than one bucket was signalled; the priority above decided.
        others = ", ".join(
            f"{age_range}<-{evidence}" for age_range, evidence in matches[1:]
        )
        return winner[0], f"{winner[1]} (also {others})"
    return winner
