import { writable } from 'svelte/store'

// Latest snapshot pushed by the daemon over SSE, plus a connection indicator.
export const snapshot = writable(null)
export const connection = writable('connecting')

// startStream subscribes to /api/stream; returns an unsubscribe function.
export function startStream() {
  const es = new EventSource('/api/stream')
  es.addEventListener('snapshot', (e) => {
    snapshot.set(JSON.parse(e.data))
    connection.set('open')
  })
  es.onerror = () => connection.set('error')
  return () => es.close()
}

const json = (r) => r.json()
export const getRules = () => fetch('/api/rules').then(json)
export const setACL = (ruleset) =>
  fetch('/api/acl', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ json: ruleset }),
  }).then(json)
export const reload = () => fetch('/api/reload', { method: 'POST' }).then(json)
export const resetStats = () => fetch('/api/reset', { method: 'POST' }).then(json)

export function protoName(p) {
  return { 1: 'icmp', 6: 'tcp', 17: 'udp', 58: 'icmp6' }[p] || (p < 0 ? '-' : String(p))
}
