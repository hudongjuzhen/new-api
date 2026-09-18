/*
Copyright (C) 2023-2026 QuantumNous

Unit tests for the app-center API samples. The snippet text is what an
integrator copies, so the endpoint paths, the credential header and the value
keys it contains are a contract.
*/

import { describe, expect, it } from 'vitest'

import type { SchemaParam } from '../../api'
import {
  buildApiSample,
  buildRunBody,
  endpointUrl,
  renderRequest,
} from '../api-samples'

const BASE_URL = 'https://gateway.example.com'

const schema: SchemaParam[] = [
  {
    nodeId: '122',
    fieldName: 'prompt',
    label: '提示词',
    type: 'text',
    required: true,
  },
  {
    nodeId: '275',
    fieldName: 'reference_image',
    label: '参考图',
    type: 'image',
  },
  {
    nodeId: '293',
    fieldName: 'posture_method',
    label: '姿态计算方法',
    type: 'select',
    options: [
      { label: '方法一', value: '1' },
      { label: '方法二', value: '2' },
    ],
  },
]

describe('endpointUrl', () => {
  it('targets the app run route for the selected app', () => {
    expect(endpointUrl('run', 42, BASE_URL)).toBe(
      `${BASE_URL}/api/zsy/rh/apps/42/run`
    )
  })

  it('targets the task route with a placeholder id', () => {
    expect(endpointUrl('query', 42, BASE_URL)).toBe(
      `${BASE_URL}/api/zsy/rh/apps/task/task_xxxxxxxxxxxxxxxx`
    )
    expect(endpointUrl('cancel', 42, BASE_URL)).toBe(
      `${BASE_URL}/api/zsy/rh/apps/task/task_xxxxxxxxxxxxxxxx/cancel`
    )
  })
})

describe('buildRunBody', () => {
  it('keys values by nodeId.fieldName and uses each param default', () => {
    expect(buildRunBody(schema)).toBe(
      JSON.stringify(
        {
          values: {
            '122.prompt': 'your text',
            '275.reference_image': 'openapi/example.png',
            '293.posture_method': '1',
          },
          instanceType: 'default',
        },
        null,
        2
      )
    )
  })

  it('keeps a configured default instead of the type placeholder', () => {
    const body = buildRunBody([
      {
        nodeId: '297',
        fieldName: 'intensity',
        label: '姿态强度',
        type: 'number',
        defaultValue: '1.5',
      },
    ])
    expect(JSON.parse(body).values['297.intensity']).toBe('1.5')
  })

  it('produces an empty value map for an app without parameters', () => {
    expect(JSON.parse(buildRunBody(undefined)).values).toEqual({})
  })
})

describe('renderRequest', () => {
  const request = {
    method: 'POST' as const,
    url: `${BASE_URL}/api/zsy/rh/apps/7/run`,
    body: '{\n  "values": {}\n}',
  }

  it('renders a curl command with the API key header and body', () => {
    const code = renderRequest('curl', request)
    expect(code).toContain(`curl -X POST '${request.url}'`)
    expect(code).toContain("-H 'Authorization: Bearer sk-...'")
    expect(code).toContain("-H 'Content-Type: application/json'")
    expect(code).toContain(`-d '{`)
  })

  it('renders a python request whose body is a python literal', () => {
    const code = renderRequest('python', request)
    expect(code).toContain('import requests')
    expect(code).toContain(`"${request.url}"`)
    expect(code).toContain('json={')
  })

  it('renders a fetch call for the browser runtimes', () => {
    const code = renderRequest('javascript', request)
    expect(code).toContain(`await fetch('${request.url}'`)
    expect(code).toContain('JSON.stringify({')
  })

  it('omits the body for requests that carry none', () => {
    const code = renderRequest('curl', {
      method: 'GET',
      url: `${BASE_URL}/api/zsy/rh/apps/task/task_1`,
      body: null,
    })
    expect(code).not.toContain('Content-Type')
    expect(code).not.toContain('-d ')
  })
})

describe('buildApiSample', () => {
  it('appends the literal response shape of the endpoint', () => {
    const run = buildApiSample({
      endpoint: 'run',
      lang: 'curl',
      appId: 7,
      schema,
      baseUrl: BASE_URL,
    })
    expect(run).toContain('"taskId":"task_xxxxxxxxxxxxxxxx"')

    const cancel = buildApiSample({
      endpoint: 'cancel',
      lang: 'curl',
      appId: 7,
      baseUrl: BASE_URL,
    })
    expect(cancel).toContain('/cancel')
    expect(cancel).toContain('"refunded":true')
  })

  it('sends no body for query and cancel', () => {
    for (const endpoint of ['query', 'cancel'] as const) {
      const code = buildApiSample({
        endpoint,
        lang: 'javascript',
        appId: 7,
        schema,
        baseUrl: BASE_URL,
      })
      expect(code).not.toContain('body: JSON.stringify')
    }
  })
})
