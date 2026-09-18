/*
Copyright (C) 2023-2026 QuantumNous

RunningHub app center: builders for the copy-paste API samples.

The samples are pure strings so they can be unit-tested without a DOM: they are
what a third-party integrator copies, which makes their exact content a contract
(endpoint paths, credential header, value keys). Presentation lives in
components/api-examples.tsx.
*/

import type { SchemaParam } from '../api'

export type ApiSampleEndpoint = 'run' | 'query' | 'cancel'
export type ApiSampleLang = 'curl' | 'python' | 'javascript'
export type ApiSampleMethod = 'GET' | 'POST'

export const API_SAMPLE_LANGS: ApiSampleLang[] = ['curl', 'python', 'javascript']

export const API_SAMPLE_LANG_LABELS: Record<ApiSampleLang, string> = {
  curl: 'cURL',
  python: 'Python',
  javascript: 'JavaScript',
}

export const API_SAMPLE_LANG_HIGHLIGHT: Record<
  ApiSampleLang,
  'bash' | 'python' | 'javascript'
> = {
  curl: 'bash',
  python: 'python',
  javascript: 'javascript',
}

// Placeholders stay ASCII: they are code, not UI copy.
export const API_KEY_PLACEHOLDER = 'sk-...'
export const TASK_ID_PLACEHOLDER = 'task_xxxxxxxxxxxxxxxx'
// A schema can be long; the sample stays readable when it is cut down.
const MAX_EXAMPLE_PARAMS = 12

// The literal gateway answers, shown as comments so a caller knows what to read.
const RESPONSE_COMMENT: Record<ApiSampleEndpoint, string> = {
  run: `# → {"success":true,"data":{"taskId":"${TASK_ID_PLACEHOLDER}","status":"IN_PROGRESS"}}`,
  query: `# → {"success":true,"data":{"status":"SUCCESS","result_url":"https://..."}}`,
  cancel: `# → {"success":true,"data":{"status":"FAILURE","refunded":true}}`,
}

// exampleValue prefers the param's own default and otherwise picks a neutral
// placeholder for its type, so the copied sample is valid as-is.
export function exampleValue(param: SchemaParam): string {
  if (param.defaultValue) return param.defaultValue
  switch (param.type) {
    case 'image':
      return 'openapi/example.png'
    case 'video':
      return 'openapi/example.mp4'
    case 'audio':
      return 'openapi/example.mp3'
    case 'number':
      return '1'
    case 'boolean':
      return 'true'
    case 'select':
      return param.options?.[0]?.value ?? '1'
    default:
      return 'your text'
  }
}

// buildRunBody mirrors the run endpoint's contract: values keyed by
// nodeId.fieldName, exactly how the backend reads them.
export function buildRunBody(schema: SchemaParam[] | undefined): string {
  const values: Record<string, string> = Object.fromEntries(
    (schema ?? [])
      .slice(0, MAX_EXAMPLE_PARAMS)
      .map((param) => [`${param.nodeId}.${param.fieldName}`, exampleValue(param)])
  )
  return JSON.stringify({ values, instanceType: 'default' }, null, 2)
}

export function endpointUrl(
  endpoint: ApiSampleEndpoint,
  appId: number,
  baseUrl: string
): string {
  if (endpoint === 'query') {
    return `${baseUrl}/api/zsy/rh/apps/task/${TASK_ID_PLACEHOLDER}`
  }
  if (endpoint === 'cancel') {
    return `${baseUrl}/api/zsy/rh/apps/task/${TASK_ID_PLACEHOLDER}/cancel`
  }
  return `${baseUrl}/api/zsy/rh/apps/${appId}/run`
}

// renderRequest renders one request for one language. The JSON body doubles as a
// Python literal because every example value is a string.
export function renderRequest(
  lang: ApiSampleLang,
  request: { method: ApiSampleMethod; url: string; body: string | null }
): string {
  const { method, url, body } = request

  if (lang === 'curl') {
    const parts = [`-H 'Authorization: Bearer ${API_KEY_PLACEHOLDER}'`]
    if (body) {
      parts.push(`-H 'Content-Type: application/json'`)
      parts.push(`-d '${body.replaceAll('\n', '\n  ')}'`)
    }
    return `curl -X ${method} '${url}' \\\n  ${parts.join(' \\\n  ')}`
  }

  if (lang === 'python') {
    const bodyArg = body ? `,\n    json=${body.replaceAll('\n', '\n    ')}` : ''
    return [
      'import requests',
      '',
      'response = requests.request(',
      `    "${method}",`,
      `    "${url}",`,
      `    headers={"Authorization": "Bearer ${API_KEY_PLACEHOLDER}"}${bodyArg},`,
      ')',
      'print(response.json())',
    ].join('\n')
  }

  const init = body
    ? `{\n  method: '${method}',\n  headers: {\n    Authorization: 'Bearer ${API_KEY_PLACEHOLDER}',\n    'Content-Type': 'application/json',\n  },\n  body: JSON.stringify(${body}),\n}`
    : `{\n  method: '${method}',\n  headers: { Authorization: 'Bearer ${API_KEY_PLACEHOLDER}' },\n}`
  return [
    `const response = await fetch('${url}', ${init})`,
    '',
    'const data = await response.json()',
    'console.log(data)',
  ].join('\n')
}

// buildApiSample is the full snippet one tab shows: the request plus the literal
// response shape it yields.
export function buildApiSample(options: {
  endpoint: ApiSampleEndpoint
  lang: ApiSampleLang
  appId: number
  schema?: SchemaParam[]
  baseUrl: string
}): string {
  const { endpoint, lang, appId, schema, baseUrl } = options
  const request = {
    method: (endpoint === 'query' ? 'GET' : 'POST') as ApiSampleMethod,
    url: endpointUrl(endpoint, appId, baseUrl),
    body: endpoint === 'run' ? buildRunBody(schema) : null,
  }
  return `${renderRequest(lang, request)}\n\n${RESPONSE_COMMENT[endpoint]}`
}
