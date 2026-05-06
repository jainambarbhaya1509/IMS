import axios from 'axios'

// All backend calls go through here.
// If you change the backend URL, change it once here.
const BASE = 'http://localhost:8080'

const api = axios.create({ baseURL: BASE })

// Fetch all work items (hits Redis first, falls back to Postgres)
export const getWorkItems = () => api.get('/work-items').then(r => r.data || [])

// Fetch raw signals for one incident (from MongoDB)
export const getSignals = (id) => api.get(`/work-items/${id}/signals`).then(r => r.data || [])

// Transition status: OPEN → INVESTIGATING → RESOLVED → CLOSED
export const transitionStatus = (id, status) =>
  api.patch(`/work-items/${id}/status`, { status })

// Submit RCA for an incident
export const submitRCA = (id, rca) =>
  api.post(`/work-items/${id}/rca`, rca)

// Timeseries: signals per hour per component
export const getMetrics = () =>
  api.get('/metrics/signals-per-hour').then(r => r.data || [])

// Health check
export const getHealth = () => api.get('/health').then(r => r.data)