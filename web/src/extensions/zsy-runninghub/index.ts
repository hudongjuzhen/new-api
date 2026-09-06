/*
Copyright (C) 2023-2026 QuantumNous

RunningHub plugin frontend registration.

Registers sidebar menu entries into the host's extension anchor
(EXT_MENU_GROUPS). Translations live in the core locale files
(src/i18n/locales/*.json) so the host i18n tooling (i18n:sync,
find-missing-keys) covers them like any other UI string.
*/

import { Boxes, Sparkles } from "lucide-react";

import { EXT_MENU_GROUPS } from "@/extensions/menus";
import { ROLE } from "@/lib/roles";

// Register sidebar menu group. "RH App Center" is available to every
// authenticated user; "RH Apps" (management) is admin-only.
//
// The titles are plain-English i18n keys: the host renders extension menu
// titles through t() (see hooks/use-sidebar-data.ts), so these must stay
// English strings that exist in src/i18n/locales/*.json. Never call
// useTranslation() here — this module is evaluated outside React's render
// context, which crashes the whole app at startup.
EXT_MENU_GROUPS.push({
  id: "zsy-runninghub",
  title: "RunningHub",
  items: [
    {
      title: "RH App Center",
      url: "/rh-app-center",
      icon: Sparkles,
    },
    {
      title: "RH Apps",
      url: "/rh-apps",
      icon: Boxes,
      requiredRole: ROLE.ADMIN,
    },
  ],
});
