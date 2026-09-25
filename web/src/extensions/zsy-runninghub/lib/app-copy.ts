/*
Copyright (C) 2023-2026 QuantumNous

RunningHub admin app drafts: build the create-DTO the app form submits from a
stored app row, either verbatim (edit) or as a duplicated record (copy).

App names carry a unique index in the backend, so a duplicated record must
arrive with a fresh name before it can be saved; everything else is copied as
is and left to the admin to adjust in the form.
*/

import type { AppCreateDTO, AppView, SchemaParam } from '../api'

/** Suffix appended to the source name of a duplicated record. */
const COPY_NAME_SUFFIX = '_copy'

/** Draft params own their array, so editing a copy never touches the row the
 * list query cached. */
function cloneParamSchema(params: SchemaParam[]): SchemaParam[] {
  return params.map((param) =>
    param.options
      ? { ...param, options: param.options.map((option) => ({ ...option })) }
      : { ...param }
  )
}

/** Faithful draft of a stored app, shaped as the create/update payload. */
export function appToCreateDTO(app: AppView): AppCreateDTO {
  return {
    name: app.name,
    slug: app.slug,
    kind: app.kind,
    upstreamId: app.upstreamId,
    description: app.description,
    coverUrl: app.coverUrl,
    published: app.published,
    adminOnly: app.adminOnly,
    paramSchema: cloneParamSchema(app.paramSchema),
    perCallBilling: app.perCallBilling,
    fixedQuotaPerCall: app.fixedQuotaPerCall,
    perSecondBilling: app.perSecondBilling,
    quotaPerSecond: app.quotaPerSecond,
    secondsExpr: app.secondsExpr ?? '',
    modelBaseRateRatio: app.modelBaseRateRatio,
    site: app.site ?? '',
    categoryId: app.categoryId ?? null,
  }
}

/** `_copy` name for the source, with a counter when that name is taken too.
 * Comparison is case-insensitive because MySQL's default collation treats
 * names that differ only in case as duplicates. */
function nextCopyName(sourceName: string, existingNames: readonly string[]) {
  const taken = new Set(existingNames.map((name) => name.trim().toLowerCase()))
  const base = `${sourceName}${COPY_NAME_SUFFIX}`
  let candidate = base
  let counter = 2
  while (taken.has(candidate.toLowerCase())) {
    candidate = `${base}${counter}`
    counter += 1
  }
  return candidate
}

/**
 * Draft for duplicating a stored app: same content, new name. `existingNames`
 * are the names already shown in the admin list, so repeated copies of one app
 * come out as `x_copy`, `x_copy2`, … instead of colliding on save.
 */
export function appCopyDraft(
  app: AppView,
  existingNames: readonly string[]
): AppCreateDTO {
  const draft = appToCreateDTO(app)
  draft.name = nextCopyName(app.name, existingNames)
  return draft
}
