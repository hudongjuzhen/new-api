/*
Copyright (C) 2023-2026 QuantumNous

RunningHub Apps admin page: list, create, edit, copy, delete.

The app form uses a two-column layout — basic info on the left, the
parameter-template editor on the right (with a one-click fetch that calls
RunningHub's apiCallDemo through the selected channel). "Copy" opens the same
form pre-filled with an existing record (new name, everything else identical)
so a variant can be created without retyping the parameter template.
*/

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Plus,
  Pencil,
  Copy,
  Trash2,
  Wand2,
  Loader2,
  FolderOpen,
  GripVertical,
} from 'lucide-react'
import { useState, type DragEvent, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { uploadPlaygroundImage } from '@/features/playground/api'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import {
  getEditableQuotaStep,
  parseQuotaFromDollars,
  quotaUnitsToEditableAmount,
} from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  listApps,
  createApp,
  updateApp,
  deleteApp,
  reorderApps,
  parseCurlRequest,
  listSites,
  listCategories,
  createCategory,
  updateCategory,
  deleteCategory,
  type AppCategory,
  type AppListResult,
  type CategoryInput,
  type AppView,
  type AppCreateDTO,
  type SiteSpec,
  type SiteUpstream,
  type SiteUpstreamMap,
} from '../api'
import { CoverImageField } from '../components/cover-image-field'
import { ParamSchemaEditor } from '../components/param-schema-editor'
import { appCopyDraft, appToCreateDTO } from '../lib/app-copy'

/** RunningHub's compute instances. The values are the exact tokens the RH
 * submit body carries, so they are never translated or re-spelled; kept in sync
 * with the instance types validateApp accepts. */
const INSTANCE_TYPE_CHOICES = ['default', 'plus', 'ultra']

/** Instance an app without a setting runs on. */
const INSTANCE_TYPE_DEFAULT = 'default'

/**
 * One identifier input pair to render. `label` is already translated: the site
 * label doubles as an i18n key, and the pair legend/fields are composed from it.
 */
type SiteBlock = { key: string; label: string }

/**
 * The blocks the form offers: every site the backend registry knows, plus any
 * site a stored app already carries an identifier for.
 *
 * The second half matters when the registry cannot be read (an older backend,
 * a failed request): the app's own values must stay editable even then, so a
 * save never silently drops an upstream id it could not render.
 */
function siteBlocksFrom(
  sites: SiteSpec[],
  stored: SiteUpstreamMap
): SiteBlock[] {
  const blocks: SiteBlock[] = sites.map((site) => ({
    key: site.key,
    label: site.label,
  }))
  for (const key of Object.keys(stored)) {
    if (!blocks.some((block) => block.key === key)) {
      blocks.push({ key, label: key })
    }
  }
  return blocks
}

/** The app's identifier on the site it runs on, falling back to the primary. */
function upstreamIdOf(app: AppView): string {
  return app.siteUpstreams?.[app.site]?.upstreamId || app.upstreamId
}

const emptyDTO: AppCreateDTO = {
  name: '',
  slug: '',
  kind: 'ai_app',
  upstreamId: '',
  siteUpstreams: {},
  description: '',
  coverUrl: '',
  published: true,
  adminOnly: false,
  paramSchema: [],
  perCallBilling: false,
  fixedQuotaPerCall: 0,
  perSecondBilling: false,
  quotaPerSecond: 0,
  secondsExpr: '',
  perCharBilling: false,
  quotaPerChar: 0,
  charCountExpr: '',
  modelBaseRateRatio: 1.0,
  instanceType: INSTANCE_TYPE_DEFAULT,
  site: '',
  categoryId: null,
}

/** Billing mode selector value → the mutually-exclusive flags. */
type BillingMode = 'dynamic' | 'per-call' | 'per-second' | 'per-char'

const BILLING_MODE_CHOICES: BillingMode[] = [
  'dynamic',
  'per-call',
  'per-second',
  'per-char',
]

const BILLING_MODE_LABEL: Record<BillingMode, string> = {
  dynamic: 'Dynamic Billing',
  'per-call': 'Per-Call Billing',
  'per-second': 'Per-Second Billing',
  'per-char': 'Per-Character Billing',
}

/** What the form dialog is doing: blank / editing a record / duplicating one. */
type FormMode = 'create' | 'edit' | 'copy'

const FORM_TITLE: Record<FormMode, string> = {
  create: 'Create App',
  edit: 'Edit App',
  copy: 'Copy App',
}

function billingModeOf(d: {
  perCallBilling: boolean
  perSecondBilling: boolean
  perCharBilling: boolean
}): BillingMode {
  if (d.perCallBilling) return 'per-call'
  if (d.perSecondBilling) return 'per-second'
  if (d.perCharBilling) return 'per-char'
  return 'dynamic'
}

const CURL_EXAMPLE = `curl --location --request POST 'https://www.runninghub.cn/openapi/v2/run/ai-app/2051268528824700930' \\
--header "Content-Type: application/json" \\
--header "Authorization: Bearer \${RUNNINGHUB_API_KEY}" \\
--data-raw '{
  "nodeInfoList": [
    {
      "nodeId": "642",
      "fieldName": "image",
      "fieldValue": "4cec23f9fef05cb20ca8b045fdf6daee81016b3d69e3dcc0b99430e1610fa3d3.jpg",
      "description": "image"
    },
    {
      "nodeId": "641",
      "fieldName": "text",
      "fieldValue": "8K高清，人像精修，去除脸上的痘印，专业人像摄影，肤色干净通透，均匀细腻无瑕疵，奶油肌质感，专业影棚精修，柔和影棚光，高清细节，增强头发质感纹理，整个人看起来非常有气色，清晰可见的发丝，超高清的服装，整体色调与原图保持一致",
      "description": "提示词"
    },
    {
      "nodeId": "755",
      "fieldName": "value",
      "fieldValue": "2048",
      "description": "尺寸"
    }
  ],
  "instanceType": "default",
  "usePersonalQueue": "false"
}'`

function AppForm({
  initial,
  onSubmit,
  onCancel,
}: {
  initial: AppCreateDTO
  onSubmit: (dto: AppCreateDTO) => void
  onCancel: () => void
}) {
  const { t } = useTranslation()
  const [dto, setDto] = useState<AppCreateDTO>(initial)
  const [curlInput, setCurlInput] = useState('')
  const [fetching, setFetching] = useState(false)

  const { data: sites = [] } = useQuery({
    queryKey: ['rh-sites'],
    queryFn: () => listSites(),
  })
  const { data: categories = [] } = useQuery({
    queryKey: ['rh-app-categories'],
    queryFn: () => listCategories(),
  })
  const catId = dto.categoryId ?? null
  const siteBlocks = siteBlocksFrom(sites, dto.siteUpstreams)

  // Editable billing fields are shown in the host's configured currency (USD /
  // CNY / custom) or tokens. The stored value stays in quota units — inputs
  // convert on mount/edit, submit converts back (same helpers the redemption
  // and user-quota forms use).
  const isTokensMode = getCurrencyDisplay().meta.kind === 'tokens'
  const quotaToDisplay = (quota: number) =>
    isTokensMode ? quota : quotaUnitsToEditableAmount(quota)
  const displayToQuota = (value: number) =>
    isTokensMode ? Math.round(value) : parseQuotaFromDollars(value)

  const set = <K extends keyof AppCreateDTO>(k: K, v: AppCreateDTO[K]) =>
    setDto((prev) => ({ ...prev, [k]: v }))

  /** Write one field of one site's identifier pair, keeping the other field. */
  const setSiteUpstream = (
    key: string,
    field: keyof SiteUpstream,
    value: string
  ) =>
    setDto((prev) => {
      const current: SiteUpstream = prev.siteUpstreams[key] ?? {
        upstreamId: '',
        slug: '',
      }
      return {
        ...prev,
        siteUpstreams: {
          ...prev.siteUpstreams,
          [key]: { ...current, [field]: value },
        },
      }
    })

  const handleFetch = async () => {
    if (!curlInput.trim()) {
      toast.error(t('Please paste a RunningHub request example first'))
      return
    }
    setFetching(true)
    try {
      const res = await parseCurlRequest(curlInput.trim())
      const data = res.data
      if (!res.success || !data) {
        toast.error(res.message || t('Failed to parse the request example'))
        return
      }
      const schema = data.schema ?? []
      const errors = data.schemaErrors ?? []
      setDto((prev) => {
        // The imported request belongs to the site its host belongs to, so its
        // id is filed under that site's block. Without a recognised host the
        // id goes to the site the form already has selected (or the first one),
        // because an identifier stored under the wrong site would be submitted
        // to that site's endpoint.
        const target = data.site || prev.site || siteBlocks[0]?.key || ''
        const next: AppCreateDTO = {
          ...prev,
          kind: data.kind || prev.kind,
          // An import also selects the site it came from, so a curl pasted into
          // a fresh form fills in everything the run needs.
          site: prev.site || data.site || '',
          paramSchema: schema,
        }
        if (target) {
          const stored = prev.siteUpstreams[target]
          next.siteUpstreams = {
            ...prev.siteUpstreams,
            [target]: {
              upstreamId: data.upstreamId || stored?.upstreamId || '',
              // The upstream id goes into the identifier (标识符) so names stay
              // human-chosen and unique; a curl cannot know the app name.
              slug: stored?.slug?.trim() || data.slug || '',
            },
          }
        }
        return next
      })
      if (errors.length > 0) {
        toast.warning(t('Request example parsed, but some fields were skipped'))
      } else {
        toast.success(t('Request example parsed'))
      }
    } catch (e) {
      toast.error(String((e as Error)?.message ?? e))
    } finally {
      setFetching(false)
    }
  }

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit(dto)
      }}
      className='space-y-4'
    >
      <div className='grid grid-cols-1 gap-6 lg:grid-cols-2'>
        {/* ---------- LEFT: basic info ---------- */}
        <div className='space-y-4'>
          <div className='space-y-1.5'>
            <Label htmlFor='rh-app-name'>{t('App Name')}</Label>
            <Input
              id='rh-app-name'
              value={dto.name}
              onChange={(e) => set('name', e.target.value)}
              required
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='rh-app-site'>{t('Site')}</Label>
            <Select
              value={dto.site}
              onValueChange={(v) => {
                if (v != null) set('site', v)
              }}
            >
              <SelectTrigger id='rh-app-site'>
                <SelectValue placeholder={t('Auto')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value=''>{t('Auto')}</SelectItem>
                {sites.map((site) => (
                  <SelectItem key={site.key} value={site.key}>
                    {t(site.label)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Which site handles this app. The run submits on that site with the upstream ID configured for it below.'
              )}
            </p>
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='rh-app-instance-type'>{t('Instance Type')}</Label>
            <Select
              value={dto.instanceType || INSTANCE_TYPE_DEFAULT}
              onValueChange={(v) => {
                if (v != null) set('instanceType', v)
              }}
            >
              <SelectTrigger id='rh-app-instance-type'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {INSTANCE_TYPE_CHOICES.map((c) => (
                  <SelectItem key={c} value={c}>
                    {c}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className='text-muted-foreground text-xs'>
              {t(
                'The RH compute instance this app runs on (default 24GB / plus 48GB / ultra 80GB VRAM)'
              )}
            </p>
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='rh-app-category'>{t('Category')}</Label>
            <Select
              value={catId == null ? '' : String(catId)}
              onValueChange={(v) =>
                set('categoryId', v === '' ? null : Number(v))
              }
            >
              <SelectTrigger id='rh-app-category'>
                <SelectValue placeholder={t('Uncategorized')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value=''>{t('Uncategorized')}</SelectItem>
                {categories.map((c) => (
                  <SelectItem key={c.id} value={String(c.id)}>
                    {c.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className='text-muted-foreground text-xs'>
              {t('Select a category for this app')}
            </p>
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='rh-app-kind'>{t('Kind')}</Label>
            <Select
              value={dto.kind}
              onValueChange={(v) => {
                if (v != null) set('kind', v)
              }}
            >
              <SelectTrigger id='rh-app-kind'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='ai_app'>{t('AIC App')}</SelectItem>
                <SelectItem value='workflow'>{t('Workflow')}</SelectItem>
                <SelectItem value='model'>{t('Model')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {/* One upstream id / identifier pair per site: the same workflow has a
              different id on each RunningHub deployment, and the run uses the
              pair of the site it runs on. */}
          <div className='space-y-2'>
            <div className='text-sm font-medium'>
              {t('Upstream IDs by site')}
            </div>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Each site answers to its own upstream app ID. A run uses the identifiers of the site it runs on (see Site above).'
              )}
            </p>
            {siteBlocks.map((site) => (
              <div key={site.key} className='space-y-2 rounded-md border p-3'>
                <div className='grid grid-cols-2 gap-4'>
                  <div className='space-y-1.5'>
                    <Label htmlFor={`rh-app-upstream-${site.key}`}>
                      {t('{{site}} Upstream ID', { site: t(site.label) })}
                    </Label>
                    <Input
                      id={`rh-app-upstream-${site.key}`}
                      value={dto.siteUpstreams[site.key]?.upstreamId ?? ''}
                      onChange={(e) =>
                        setSiteUpstream(site.key, 'upstreamId', e.target.value)
                      }
                      // Only the site this app runs on must carry an id; the
                      // others are optional, because an app rarely exists on
                      // every site.
                      required={dto.site === site.key}
                    />
                  </div>
                  <div className='space-y-1.5'>
                    <Label htmlFor={`rh-app-identifier-${site.key}`}>
                      {t('{{site}} Identifier', { site: t(site.label) })}
                    </Label>
                    <Input
                      id={`rh-app-identifier-${site.key}`}
                      value={dto.siteUpstreams[site.key]?.slug ?? ''}
                      onChange={(e) =>
                        setSiteUpstream(site.key, 'slug', e.target.value)
                      }
                    />
                  </div>
                </div>
              </div>
            ))}
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='rh-app-billing-mode'>{t('Billing Mode')}</Label>
            <Select
              value={billingModeOf(dto)}
              onValueChange={(v) => {
                const mode = v as BillingMode
                set('perCallBilling', mode === 'per-call')
                set('perSecondBilling', mode === 'per-second')
                set('perCharBilling', mode === 'per-char')
              }}
            >
              <SelectTrigger id='rh-app-billing-mode'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {BILLING_MODE_CHOICES.map((m) => (
                  <SelectItem key={m} value={m}>
                    {t(BILLING_MODE_LABEL[m])}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Per-second apps charge by the seconds/duration parameter value; per-call apps charge a flat quota per run.'
              )}
            </p>
          </div>
          {billingModeOf(dto) === 'dynamic' && (
            <div className='space-y-1.5'>
              <Label htmlFor='rh-app-ratio'>{t('Model Base Rate Ratio')}</Label>
              <Input
                id='rh-app-ratio'
                type='number'
                step='0.01'
                min='0.01'
                value={dto.modelBaseRateRatio}
                onChange={(e) =>
                  set(
                    'modelBaseRateRatio',
                    Number.parseFloat(e.target.value) || 1.0
                  )
                }
              />
            </div>
          )}
          {billingModeOf(dto) === 'per-call' && (
            <div className='space-y-1.5'>
              <Label htmlFor='rh-app-fixed'>
                {t('Fixed Quota ({{currency}})', {
                  currency: isTokensMode ? t('Tokens') : getCurrencyLabel(),
                })}
              </Label>
              <Input
                id='rh-app-fixed'
                type='number'
                min='0'
                step={getEditableQuotaStep()}
                value={quotaToDisplay(dto.fixedQuotaPerCall)}
                onChange={(e) =>
                  set(
                    'fixedQuotaPerCall',
                    displayToQuota(Number(e.target.value) || 0)
                  )
                }
              />
            </div>
          )}
          {billingModeOf(dto) === 'per-second' && (
            <div className='space-y-1.5'>
              <Label htmlFor='rh-app-per-second'>
                {t('Quota Per Second ({{currency}})', {
                  currency: isTokensMode ? t('Tokens') : getCurrencyLabel(),
                })}
              </Label>
              <Input
                id='rh-app-per-second'
                type='number'
                min='0'
                step={getEditableQuotaStep()}
                value={quotaToDisplay(dto.quotaPerSecond)}
                onChange={(e) =>
                  set(
                    'quotaPerSecond',
                    displayToQuota(Number(e.target.value) || 0)
                  )
                }
              />
              <Label htmlFor='rh-app-seconds-expr'>{t('Seconds Field')}</Label>
              <Input
                id='rh-app-seconds-expr'
                value={dto.secondsExpr}
                placeholder='nodeId=212 / 229-212'
                onChange={(e) => set('secondsExpr', e.target.value)}
              />
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Seconds field expression over node ids, e.g. "212" or "229-212".'
                )}
              </p>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Leave empty to use a seconds/duration parameter; the result is clamped to 1-3600 seconds.'
                )}
              </p>
            </div>
          )}
          {billingModeOf(dto) === 'per-char' && (
            <div className='space-y-1.5'>
              <Label htmlFor='rh-app-per-char'>
                {t('Quota Per Character ({{currency}})', {
                  currency: isTokensMode ? t('Tokens') : getCurrencyLabel(),
                })}
              </Label>
              <Input
                id='rh-app-per-char'
                type='number'
                min='0'
                step={getEditableQuotaStep()}
                value={quotaToDisplay(dto.quotaPerChar)}
                onChange={(e) =>
                  set(
                    'quotaPerChar',
                    displayToQuota(Number(e.target.value) || 0)
                  )
                }
              />
              <Label htmlFor='rh-app-char-count-expr'>
                {t('Character Field')}
              </Label>
              <Input
                id='rh-app-char-count-expr'
                value={dto.charCountExpr}
                placeholder='nodeId=212 / len(212)'
                onChange={(e) => set('charCountExpr', e.target.value)}
              />
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Character field expression over node ids, e.g. "212", "nodeId=212" or "len(122) + len(123)".'
                )}
              </p>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'The billed run is the character count of the named text field; an unresolvable field rejects the run instead of guessing.'
                )}
              </p>
            </div>
          )}
          <div className='space-y-1.5'>
            <Label htmlFor='rh-app-desc'>{t('Description')}</Label>
            <Input
              id='rh-app-desc'
              value={dto.description}
              onChange={(e) => set('description', e.target.value)}
            />
          </div>
          <CoverImageField
            value={dto.coverUrl}
            onChange={(coverUrl) => set('coverUrl', coverUrl)}
            uploadImage={uploadPlaygroundImage}
          />
          <div className='flex items-center gap-6'>
            <label className='flex items-center gap-2'>
              <Switch
                checked={dto.published}
                onCheckedChange={(v) => set('published', v as boolean)}
              />
              <span className='text-sm'>{t('Published')}</span>
            </label>
            <label className='flex items-center gap-2'>
              <Switch
                checked={dto.adminOnly}
                onCheckedChange={(v) => set('adminOnly', v as boolean)}
              />
              <span className='text-sm'>{t('Admin Only')}</span>
            </label>
          </div>
        </div>

        {/* ---------- RIGHT: request example + parameter template ---------- */}
        <div className='space-y-3 rounded-md border p-3'>
          <div className='space-y-1.5'>
            <Label htmlFor='rh-app-curl'>{t('Request Example')}</Label>
            <textarea
              id='rh-app-curl'
              className='text-muted-foreground field-sizing-content max-h-72 min-h-40 w-full rounded-md border bg-transparent px-2 py-1 font-mono text-xs'
              placeholder={t('Paste a RunningHub curl request example…')}
              value={curlInput}
              onChange={(e) => setCurlInput(e.target.value)}
            />
          </div>
          <div className='flex items-center justify-between gap-2'>
            <Button
              type='button'
              size='sm'
              variant='ghost'
              title={t('Fill with an example')}
              onClick={() => setCurlInput(CURL_EXAMPLE)}
            >
              {t('Use example')}
            </Button>
            <Button
              type='button'
              size='sm'
              onClick={handleFetch}
              disabled={fetching}
            >
              {fetching ? (
                <Loader2 className='size-4 animate-spin' />
              ) : (
                <Wand2 className='size-4' />
              )}
              {t('Fetch Template')}
            </Button>
          </div>
          <div className='flex items-center justify-between'>
            <div className='text-sm font-medium'>{t('Parameter Template')}</div>
          </div>
          <ParamSchemaEditor
            params={dto.paramSchema}
            onChange={(paramSchema) => set('paramSchema', paramSchema)}
          />
        </div>
      </div>

      <div className='flex justify-end gap-2 pt-2'>
        <Button type='button' variant='outline' onClick={onCancel}>
          {t('Cancel')}
        </Button>
        <Button type='submit'>{t('Save')}</Button>
      </div>
    </form>
  )
}

function siteLabel(site: string, sites: SiteSpec[]): string {
  const known = sites.find((spec) => spec.key === site)
  if (known) return known.label
  return site || '—'
}

function CategoryBadge({ name }: { name: string }) {
  return <Badge variant='outline'>{name}</Badge>
}

function billingBadge(app: AppView, t: (key: string) => string) {
  if (app.perCallBilling) {
    return <Badge>{t('Per-Call Billing')}</Badge>
  }
  if (app.perSecondBilling) {
    return <Badge variant='outline'>{t('Per-Second Billing')}</Badge>
  }
  if (app.perCharBilling) {
    return <Badge variant='outline'>{t('Per-Character Billing')}</Badge>
  }
  return <Badge variant='outline'>{t('Dynamic')}</Badge>
}

/** Which side of the hovered row the dragged app would land on. */
type DropPosition = 'before' | 'after'

/** The list's in-flight drag, owned by the page so the body stays presentational. */
type RowDragState = {
  draggedId: number | null
  dropTargetId: number | null
  dropPosition: DropPosition
}

const IDLE_DRAG: RowDragState = {
  draggedId: null,
  dropTargetId: null,
  dropPosition: 'before',
}

/** Table width the loading/empty rows have to span. */
const APP_TABLE_COLUMNS = 11

/**
 * Apps table body. Every row is draggable and reordering is delegated to the
 * page; the drop side is drawn on the row the app would land next to. The
 * handle also carries the keyboard equivalent (ArrowUp/ArrowDown), because a
 * drag is not reachable without a pointer.
 */
function AppsTableBody(props: {
  isLoading: boolean
  data: AppListResult | undefined
  sites: SiteSpec[]
  drag: RowDragState
  onEdit: (app: AppView) => void
  onCopy: (app: AppView) => void
  onDelete: (id: number) => void
  onDragStart: (event: DragEvent<HTMLTableRowElement>, id: number) => void
  onDragOver: (event: DragEvent<HTMLTableRowElement>, id: number) => void
  onDrop: (event: DragEvent<HTMLTableRowElement>, id: number) => void
  onDragEnd: () => void
  onMove: (id: number, direction: -1 | 1) => void
}) {
  const { t } = useTranslation()

  if (props.isLoading) {
    return (
      <TableRow>
        <TableCell colSpan={APP_TABLE_COLUMNS} className='text-center'>
          {t('Loading...')}
        </TableCell>
      </TableRow>
    )
  }
  const items = props.data?.items ?? []
  if (items.length === 0) {
    return (
      <TableRow>
        <TableCell colSpan={APP_TABLE_COLUMNS} className='text-center'>
          {t('No data')}
        </TableCell>
      </TableRow>
    )
  }

  return items.map((app) => {
    const isDragging = props.drag.draggedId === app.id
    const isDropTarget =
      props.drag.dropTargetId === app.id && props.drag.draggedId !== app.id
    return (
      <TableRow
        key={app.id}
        draggable
        onDragStart={(event) => props.onDragStart(event, app.id)}
        onDragOver={(event) => props.onDragOver(event, app.id)}
        onDrop={(event) => props.onDrop(event, app.id)}
        onDragEnd={props.onDragEnd}
        className={cn(
          'transition-opacity',
          isDragging && 'opacity-50',
          isDropTarget &&
            props.drag.dropPosition === 'before' &&
            'border-t-primary border-t-2',
          isDropTarget &&
            props.drag.dropPosition === 'after' &&
            'border-b-primary border-b-2'
        )}
      >
        <TableCell className='w-10 pr-0'>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            className='cursor-grab active:cursor-grabbing'
            title={t('Drag to reorder')}
            // The label names the app, so a screen reader hears which row the
            // handle moves instead of the same words on every row.
            aria-label={t('Drag {{group}} to reorder', { group: app.name })}
            onKeyDown={(event) => {
              if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return
              event.preventDefault()
              props.onMove(app.id, event.key === 'ArrowUp' ? -1 : 1)
            }}
          >
            <GripVertical className='size-3.5' />
          </Button>
        </TableCell>
        <TableCell>{app.id}</TableCell>
        <TableCell>{app.sort}</TableCell>
        <TableCell className='font-medium'>{app.name}</TableCell>
        <TableCell>
          {app.categoryName ? (
            <Badge variant='outline'>{app.categoryName}</Badge>
          ) : (
            <span className='text-muted-foreground/60 text-xs'>-</span>
          )}
        </TableCell>
        <TableCell>{app.kind}</TableCell>
        <TableCell className='font-mono text-xs'>{upstreamIdOf(app)}</TableCell>
        <TableCell className='text-xs'>
          {t(siteLabel(app.site, props.sites))}
        </TableCell>
        {/* The upstream token (default / plus / ultra) is shown as-is: it is
            what the RH request body carries, not a localized label. */}
        <TableCell className='font-mono text-xs'>
          {app.instanceType || INSTANCE_TYPE_DEFAULT}
        </TableCell>
        <TableCell>{billingBadge(app, t)}</TableCell>
        <TableCell className='text-right'>
          <Button
            variant='ghost'
            size='icon-sm'
            onClick={() => props.onEdit(app)}
            title={t('Edit')}
            aria-label={t('Edit')}
          >
            <Pencil className='size-3.5' />
          </Button>
          <Button
            variant='ghost'
            size='icon-sm'
            onClick={() => props.onCopy(app)}
            title={t('Copy App')}
            aria-label={t('Copy App')}
          >
            <Copy className='size-3.5' />
          </Button>
          <Button
            variant='ghost'
            size='icon-sm'
            onClick={() => props.onDelete(app.id)}
            title={t('Delete')}
            aria-label={t('Delete')}
          >
            <Trash2 className='size-3.5' />
          </Button>
        </TableCell>
      </TableRow>
    )
  })
}

export function CategoryManageDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState<AppCategory | null>(null)
  const [name, setName] = useState('')
  const [sortOrder, setSortOrder] = useState(0)
  const [deleteId, setDeleteId] = useState<number | null>(null)

  const { data: categories = [], isLoading } = useQuery({
    queryKey: ['rh-app-categories'],
    queryFn: () => listCategories(),
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['rh-app-categories'] })
    queryClient.invalidateQueries({ queryKey: ['rh-apps'] })
  }

  const startEdit = (cat: AppCategory) => {
    setEditing(cat)
    setName(cat.name)
    setSortOrder(cat.sortOrder ?? 0)
  }
  const startCreate = () => {
    setEditing(null)
    setName('')
    setSortOrder(0)
  }

  const save = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const input: CategoryInput = { name: name.trim(), sortOrder }
    if (!input.name) return
    const mutate = editing
      ? updateCategory(editing.id, input)
      : createCategory(input)
    mutate
      .then(() => {
        toast.success(editing ? t('Category updated') : t('Category created'))
        invalidate()
        startCreate()
      })
      .catch((err: unknown) =>
        toast.error(String((err as Error)?.message ?? err))
      )
  }

  const confirmDelete = () => {
    if (deleteId == null) return
    deleteCategory(deleteId)
      .then(() => {
        toast.success(t('Category deleted'))
        invalidate()
        setDeleteId(null)
      })
      .catch((err: unknown) => {
        toast.error(String((err as Error)?.message ?? err))
        setDeleteId(null)
      })
  }

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className='sm:max-w-lg'>
          <DialogHeader>
            <DialogTitle>{t('Category Management')}</DialogTitle>
          </DialogHeader>
          <div className='space-y-4'>
            <form onSubmit={save} className='flex items-end gap-2'>
              <div className='flex-1 space-y-1.5'>
                <Label htmlFor='rh-cat-name'>{t('Category Name')}</Label>
                <Input
                  id='rh-cat-name'
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder={t('New category name…')}
                />
              </div>
              <div className='w-24 space-y-1.5'>
                <Label htmlFor='rh-cat-order'>{t('Sort')}</Label>
                <Input
                  id='rh-cat-order'
                  type='number'
                  value={sortOrder}
                  onChange={(e) => setSortOrder(Number(e.target.value) || 0)}
                />
              </div>
              <Button type='submit'>{editing ? t('Save') : t('Add')}</Button>
            </form>

            <div className='max-h-72 space-y-1 overflow-y-auto'>
              {isLoading ? (
                <div className='text-muted-foreground py-4 text-center text-sm'>
                  {t('Loading...')}
                </div>
              ) : (
                (() => {
                  if (categories.length === 0) {
                    return (
                      <div className='text-muted-foreground py-4 text-center text-sm'>
                        {t('No categories yet')}
                      </div>
                    )
                  }
                  return categories.map((cat) => (
                    <div
                      key={cat.id}
                      className='border-border/60 bg-muted/20 flex items-center justify-between gap-2 rounded-md border px-2 py-1.5'
                    >
                      <div className='flex min-w-0 items-center gap-2'>
                        <span className='text-sm font-medium'>{cat.name}</span>
                        <CategoryBadge name={`${cat.appCount ?? 0}`} />
                      </div>
                      <div className='flex shrink-0 items-center gap-1'>
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon-sm'
                          onClick={() => startEdit(cat)}
                          title={t('Edit')}
                        >
                          <Pencil className='size-3.5' />
                        </Button>
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon-sm'
                          onClick={() => setDeleteId(cat.id)}
                          title={t('Delete')}
                        >
                          <Trash2 className='size-3.5' />
                        </Button>
                      </div>
                    </div>
                  ))
                })()
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={deleteId !== null}
        onOpenChange={(v) => !v && setDeleteId(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete Category')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Deleting a category unassigns the apps in it. Existing apps are kept.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete}>
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

export function RhAppsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<AppView | null>(null)
  const [copySource, setCopySource] = useState<AppView | null>(null)
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const [catManageOpen, setCatManageOpen] = useState(false)
  const [drag, setDrag] = useState<RowDragState>(IDLE_DRAG)

  const { data, isLoading } = useQuery({
    queryKey: ['rh-apps'],
    queryFn: () => listApps({ page: 1, pageSize: 100 }),
  })
  // The registry labels the site each row runs on (and the app form renders one
  // identifier block per entry), so an operator's site list comes from the
  // backend instead of the bundle.
  const { data: sites = [] } = useQuery({
    queryKey: ['rh-sites'],
    queryFn: () => listSites(),
  })

  const apps = data?.items ?? []

  const createMutation = useMutation({
    mutationFn: (dto: AppCreateDTO) => createApp(dto),
    onSuccess: () => {
      toast.success(t('App created'))
      queryClient.invalidateQueries({ queryKey: ['rh-apps'] })
      setDialogOpen(false)
    },
    onError: (e: unknown) => {
      toast.error(String((e as Error)?.message ?? e))
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, dto }: { id: number; dto: AppCreateDTO }) =>
      updateApp(id, dto),
    onSuccess: () => {
      toast.success(t('App updated'))
      queryClient.invalidateQueries({ queryKey: ['rh-apps'] })
      setDialogOpen(false)
    },
    onError: (e: unknown) => {
      toast.error(String((e as Error)?.message ?? e))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteApp(id),
    onSuccess: () => {
      toast.success(t('App deleted'))
      queryClient.invalidateQueries({ queryKey: ['rh-apps'] })
      setDeleteId(null)
    },
    onError: (e: unknown) => {
      toast.error(String((e as Error)?.message ?? e))
    },
  })

  const reorderMutation = useMutation({
    mutationFn: (ids: number[]) => reorderApps(ids),
    onSuccess: () => toast.success(t('Sort order updated')),
    onError: (e: unknown) => {
      toast.error(String((e as Error)?.message ?? e))
    },
    onSettled: () => {
      // The server owns the final order — it renumbers every position — so the
      // list is refetched either way: on success to pick up the new sort values,
      // on failure to drop the optimistic order again.
      queryClient.invalidateQueries({ queryKey: ['rh-apps'] })
    },
  })

  /**
   * Show `ids` as the list order right away, then persist it. A drag that ends
   * where it started changes nothing and is skipped, so the round trip — and its
   * "updated" toast — belongs to real moves only.
   */
  const applyOrder = (ids: number[]) => {
    if (
      ids.length === apps.length &&
      ids.every((id, index) => id === apps[index].id)
    ) {
      return
    }
    const byId = new Map(apps.map((app) => [app.id, app]))
    const items: AppView[] = []
    for (const id of ids) {
      const app = byId.get(id)
      if (app) items.push(app)
    }
    queryClient.setQueryData<AppListResult>(['rh-apps'], (prev) =>
      prev ? { ...prev, items } : prev
    )
    reorderMutation.mutate(ids)
  }

  const resetDrag = () => setDrag(IDLE_DRAG)

  const handleDragStart = (
    event: DragEvent<HTMLTableRowElement>,
    id: number
  ) => {
    setDrag({ draggedId: id, dropTargetId: null, dropPosition: 'before' })
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(id))
  }

  const handleDragOver = (
    event: DragEvent<HTMLTableRowElement>,
    id: number
  ) => {
    event.preventDefault()
    if (drag.draggedId === null || drag.draggedId === id) return
    // The half of the row the pointer is in decides which side the app lands on.
    const rect = event.currentTarget.getBoundingClientRect()
    const dropPosition: DropPosition =
      event.clientY - rect.top > rect.height / 2 ? 'after' : 'before'
    setDrag((prev) =>
      prev.dropTargetId === id && prev.dropPosition === dropPosition
        ? prev
        : { ...prev, dropTargetId: id, dropPosition }
    )
    event.dataTransfer.dropEffect = 'move'
  }

  const handleDrop = (
    event: DragEvent<HTMLTableRowElement>,
    targetId: number
  ) => {
    event.preventDefault()
    const sourceId = drag.draggedId
    // A drop can land on a row dragover never reported (a fast release), so the
    // recorded side only counts when it belongs to this row.
    const position =
      drag.dropTargetId === targetId ? drag.dropPosition : 'before'
    resetDrag()
    if (sourceId === null || sourceId === targetId) return
    const ids = apps.map((app) => app.id)
    if (!ids.includes(sourceId) || !ids.includes(targetId)) return
    const next = ids.filter((id) => id !== sourceId)
    next.splice(
      next.indexOf(targetId) + (position === 'after' ? 1 : 0),
      0,
      sourceId
    )
    applyOrder(next)
  }

  /** Move one app one position towards the front (up) or the back (down). */
  const handleMove = (id: number, direction: -1 | 1) => {
    const ids = apps.map((app) => app.id)
    const from = ids.indexOf(id)
    const to = from + direction
    if (from < 0 || to < 0 || to >= ids.length) return
    ids.splice(to, 0, ...ids.splice(from, 1))
    applyOrder(ids)
  }

  const openCreate = () => {
    setEditing(null)
    setCopySource(null)
    setDialogOpen(true)
  }

  const openEdit = (app: AppView) => {
    setEditing(app)
    setCopySource(null)
    setDialogOpen(true)
  }

  const openCopy = (app: AppView) => {
    setEditing(null)
    setCopySource(app)
    setDialogOpen(true)
  }

  const handleSubmit = (dto: AppCreateDTO) => {
    if (editing) {
      updateMutation.mutate({ id: editing.id, dto })
    } else {
      createMutation.mutate(dto)
    }
  }

  // The form owns its state once mounted, so each target gets a fresh instance:
  // a copy opens as a create form pre-filled from the source record.
  let formMode: FormMode = 'create'
  let formInitial = emptyDTO
  if (editing) {
    formMode = 'edit'
    formInitial = appToCreateDTO(editing)
  } else if (copySource) {
    formMode = 'copy'
    formInitial = appCopyDraft(
      copySource,
      apps.map((app) => app.name)
    )
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('RunningHub Apps')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            size='sm'
            variant='outline'
            onClick={() => setCatManageOpen(true)}
          >
            <FolderOpen className='size-4' />
            {t('Category Management')}
          </Button>
          <Button size='sm' onClick={openCreate}>
            <Plus className='size-4' />
            {t('Create App')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <Table>
            <TableHeader>
              <TableRow>
                {/* Drag handle column: the header carries no label, the handle
                    itself is labelled per row. */}
                <TableHead className='w-10' />
                <TableHead>ID</TableHead>
                <TableHead title={t('Larger values are shown first')}>
                  {t('Sort')}
                </TableHead>
                <TableHead>{t('App Name')}</TableHead>
                <TableHead>{t('Category')}</TableHead>
                <TableHead>{t('Kind')}</TableHead>
                <TableHead>{t('Upstream ID')}</TableHead>
                <TableHead>{t('Site')}</TableHead>
                <TableHead>{t('Instance Type')}</TableHead>
                <TableHead>{t('Per-Call Billing')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <AppsTableBody
                isLoading={isLoading}
                data={data}
                sites={sites}
                drag={drag}
                onEdit={openEdit}
                onCopy={openCopy}
                onDelete={setDeleteId}
                onDragStart={handleDragStart}
                onDragOver={handleDragOver}
                onDrop={handleDrop}
                onDragEnd={resetDrag}
                onMove={handleMove}
              />
            </TableBody>
          </Table>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <CategoryManageDialog
        open={catManageOpen}
        onOpenChange={setCatManageOpen}
      />

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className='max-h-[85vh] w-[80vw] max-w-[80vw] overflow-y-auto sm:max-w-[80vw]'>
          <DialogHeader>
            <DialogTitle>{t(FORM_TITLE[formMode])}</DialogTitle>
            {formMode === 'copy' && copySource && (
              <DialogDescription>
                {t(
                  'Copied from {{name}}. Adjust the fields and save to create a new app.',
                  { name: copySource.name }
                )}
              </DialogDescription>
            )}
          </DialogHeader>
          <AppForm
            key={`${formMode}-${editing?.id ?? copySource?.id ?? 'new'}`}
            initial={formInitial}
            onSubmit={handleSubmit}
            onCancel={() => setDialogOpen(false)}
          />
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={deleteId !== null}
        onOpenChange={(v) => !v && setDeleteId(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete App')}</AlertDialogTitle>{' '}
            <AlertDialogDescription>
              {t('Are you sure you want to delete this app?')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteId && deleteMutation.mutate(deleteId)}
            >
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
