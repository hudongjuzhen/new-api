/*
Copyright (C) 2023-2026 QuantumNous

Extension channel-type contributions.

Merged into `features/channels/constants.ts` at the P7 anchor.
`types` maps a numeric channel id to an i18n label key (rendered with `t()`).
`displayOrder` is appended to the core `CHANNEL_TYPE_DISPLAY_ORDER` array.
*/

export interface ExtChannelTypeContribution {
  types: Record<number, string>
  displayOrder: number[]
}

export const EXT_CHANNEL_TYPES: ExtChannelTypeContribution = {
  types: {},
  displayOrder: [],
}

export const CHANNEL_TYPE_RUNNING_HUB = 61
export const CHANNEL_TYPE_RUNNING_HUB_INTL = 62
export const CHANNEL_TYPE_LIBLIB = 63
