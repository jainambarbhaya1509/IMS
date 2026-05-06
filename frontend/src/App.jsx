import { useState, useEffect, useCallback } from 'react'
import { getWorkItems } from './api/client'
import Header from './components/Header'
import IncidentList from './components/IncidentList'
import IncidentDetail from './components/IncidentDetail'
import './index.css'

export default function App() {
  const [workItems, setWorkItems] = useState([])
  const [selectedItem, setSelectedItem] = useState(null)
  const [loading, setLoading] = useState(true)

  const fetchWorkItems = useCallback(async () => {
    try {
      const items = await getWorkItems()
      setWorkItems(items)
      setSelectedItem(prev => {
        if (!prev) return null
        const updated = items.find(i => i.id === prev.id)
        return updated || prev
      })
    } catch (e) {
      console.error('Failed to fetch work items:', e)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchWorkItems() }, [fetchWorkItems])

  useEffect(() => {
    const interval = setInterval(fetchWorkItems, 5000)
    return () => clearInterval(interval)
  }, [fetchWorkItems])

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh', overflow: 'hidden' }}>
      <Header />
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden', minHeight: 0 }}>
        <div style={{
          width: 320, flexShrink: 0,
          borderRight: '1px solid var(--border)',
          overflowY: 'auto', display: 'flex', flexDirection: 'column',
        }}>
          <IncidentList
            items={workItems}
            selectedId={selectedItem?.id}
            onSelect={setSelectedItem}
            loading={loading}
          />
        </div>
        <div style={{ flex: 1, overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
          <IncidentDetail
            item={selectedItem}
            onStatusChange={fetchWorkItems}
          />
        </div>
      </div>
    </div>
  )
}