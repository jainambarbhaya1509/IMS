import { useState } from 'react'
import { submitRCA, transitionStatus } from '../api/client'

// Root cause categories — matches what the assignment specifies
const CATEGORIES = [
  'Infrastructure',
  'Configuration',
  'Code Defect',
  'Dependency Failure',
  'Network',
  'Human Error',
  'Capacity',
  'Security',
  'Unknown',
]

// RCAForm — appears when an incident is RESOLVED and needs RCA before closing
// All fields are mandatory — backend enforces this too, but we validate here first
export default function RCAForm({ workItemId, onSuccess }) {
  const [form, setForm] = useState({
    start_time: '',
    end_time: '',
    root_cause_category: '',
    fix_applied: '',
    prevention_steps: '',
  })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [step, setStep] = useState('rca') // 'rca' | 'closing'

  // Update a single form field
  const set = (key, value) => setForm(prev => ({ ...prev, [key]: value }))

  // Validate all fields before submitting
  const validate = () => {
    if (!form.start_time) return 'Incident start time is required'
    if (!form.end_time) return 'Incident end time is required'
    if (new Date(form.end_time) <= new Date(form.start_time))
      return 'End time must be after start time'
    if (!form.root_cause_category) return 'Root cause category is required'
    if (!form.fix_applied.trim()) return 'Fix applied is required'
    if (!form.prevention_steps.trim()) return 'Prevention steps are required'
    return null
  }

  const handleSubmit = async () => {
    const err = validate()
    if (err) { setError(err); return }

    setLoading(true)
    setError(null)

    try {
      // Step 1: Submit RCA
      await submitRCA(workItemId, form)
      setStep('closing')

      // Step 2: Transition to CLOSED
      await transitionStatus(workItemId, 'CLOSED')
      onSuccess()
    } catch (e) {
      // Show the error message from the backend
      setError(e.response?.data || e.message || 'Something went wrong')
      setStep('rca')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{
      background: 'var(--bg-2)',
      border: '1px solid var(--border)',
      borderRadius: 'var(--radius-lg)',
      overflow: 'hidden',
    }}>
      {/* Header */}
      <div style={{
        padding: '14px 20px',
        borderBottom: '1px solid var(--border)',
        background: 'var(--bg-3)',
        display: 'flex',
        alignItems: 'center',
        gap: 10,
      }}>
        <span style={{
          width: 8, height: 8,
          borderRadius: '50%',
          background: 'var(--p2)',
          display: 'inline-block',
        }} />
        <span style={{
          fontFamily: 'var(--font-display)',
          fontWeight: 700,
          fontSize: '13px',
          letterSpacing: '0.05em',
        }}>
          ROOT CAUSE ANALYSIS
        </span>
        <span style={{
          marginLeft: 'auto',
          fontSize: '11px',
          color: 'var(--text-3)',
        }}>
          Required to close incident
        </span>
      </div>

      {/* Form */}
      <div style={{ padding: '20px', display: 'flex', flexDirection: 'column', gap: 16 }}>

        {/* Time range row */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
          <Field label="Incident Start">
            <input
              type="datetime-local"
              value={form.start_time}
              onChange={e => set('start_time', e.target.value)}
            />
          </Field>
          <Field label="Incident End">
            <input
              type="datetime-local"
              value={form.end_time}
              onChange={e => set('end_time', e.target.value)}
            />
          </Field>
        </div>

        {/* Root cause dropdown */}
        <Field label="Root Cause Category">
          <select
            value={form.root_cause_category}
            onChange={e => set('root_cause_category', e.target.value)}
          >
            <option value="">Select a category...</option>
            {CATEGORIES.map(c => (
              <option key={c} value={c}>{c}</option>
            ))}
          </select>
        </Field>

        {/* Fix applied textarea */}
        <Field label="Fix Applied">
          <textarea
            placeholder="What was done to resolve this incident?"
            value={form.fix_applied}
            onChange={e => set('fix_applied', e.target.value)}
            style={{ minHeight: 80 }}
          />
        </Field>

        {/* Prevention steps textarea */}
        <Field label="Prevention Steps">
          <textarea
            placeholder="What will prevent this from happening again?"
            value={form.prevention_steps}
            onChange={e => set('prevention_steps', e.target.value)}
            style={{ minHeight: 80 }}
          />
        </Field>

        {/* Error message from validation or backend */}
        {error && (
          <div style={{
            background: 'var(--p0-bg)',
            border: '1px solid var(--p0-border)',
            borderRadius: 'var(--radius)',
            padding: '10px 14px',
            color: 'var(--p0)',
            fontSize: '12px',
          }}>
            ✗ {error}
          </div>
        )}

        {/* Submit button */}
        <button
          onClick={handleSubmit}
          disabled={loading}
          style={{
            background: loading ? 'var(--bg-3)' : 'var(--green)',
            color: loading ? 'var(--text-3)' : '#000',
            padding: '10px 20px',
            borderRadius: 'var(--radius)',
            fontWeight: 700,
            fontSize: '12px',
            letterSpacing: '0.08em',
            textTransform: 'uppercase',
            transition: 'opacity 0.15s',
            opacity: loading ? 0.7 : 1,
          }}
        >
          {loading
            ? (step === 'closing' ? 'Closing incident...' : 'Submitting RCA...')
            : 'Submit RCA & Close Incident'
          }
        </button>
      </div>
    </div>
  )
}

// Field — label + input wrapper
function Field({ label, children }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      <label style={{
        fontSize: '11px',
        color: 'var(--text-2)',
        fontWeight: 600,
        letterSpacing: '0.08em',
        textTransform: 'uppercase',
      }}>
        {label}
      </label>
      {children}
    </div>
  )
}