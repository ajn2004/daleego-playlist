import React, { useState, useEffect } from 'react'
import { api, apiBaseURL } from './api/client'
import type { Playlist, Series, PlaylistSeries, PlaylistSlot, PlaylistQueueItem, MediaServer, Episode, PlexPlaylistItem } from './types'
import { ShowProfileWorkbench } from './components/ShowProfileWorkbench'

export default function App() {
  const [playlists, setPlaylists] = useState<Playlist[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [series, setSeries] = useState<Series[]>([])
  const [servers, setServers] = useState<MediaServer[]>([])
  const [error, setError] = useState('')
  const [status, setStatus] = useState('')
  const [loading, setLoading] = useState(false)

  const selectedPlaylist = playlists.find(p => p.id === selectedId)

  const fetchPlaylists = async () => {
    try {
      const res = await api.playlists.list()
      setPlaylists(res.playlists || [])
    } catch (e: any) {
      setError(e.message)
    }
  }

  const fetchSeries = async () => {
    try {
      const res = await api.series.list()
      setSeries(res.series || [])
    } catch (e: any) {
      setError(e.message)
    }
  }

  const fetchServers = async () => {
    try {
      const res = await api.servers.list()
      setServers(res.media_servers || [])
    } catch (e: any) {
      setError(e.message)
    }
  }

  useEffect(() => {
    fetchPlaylists()
    fetchSeries()
    fetchServers()
  }, [])

  const handleCreated = () => {
    fetchPlaylists()
  }

  const handlePlaylistUpdate = (updatedPlaylist?: Playlist) => {
    if (!updatedPlaylist) {
      fetchPlaylists()
      return
    }
    setPlaylists(current => current.map(playlist =>
      playlist.id === updatedPlaylist.id ? updatedPlaylist : playlist
    ))
  }

  const showStatus = (msg: string) => {
    setStatus(msg)
    setTimeout(() => setStatus(''), 5000)
  }

  return (
    <div className="app-shell">
      <header className="app-header">
        <div>
          <p className="eyebrow">LOCAL PLEX CONTROL</p>
          <h1>Daleego Playlists</h1>
        </div>
        <p className="header-status"><span /> Local network</p>
      </header>

      {error && (
        <div style={{ background: '#fee', color: '#c00', padding: '0.5rem', borderRadius: 4, marginBottom: '1rem' }}>
          {error} <span style={{ fontSize: '0.85em' }}>API: {apiBaseURL}</span>
        </div>
      )}

      {status && (
        <div style={{ background: '#efe', color: '#080', padding: '0.5rem', borderRadius: 4, marginBottom: '1rem' }}>
          {status}
        </div>
      )}

      <div className="app-layout">
        <PlaylistSidebar
          playlists={playlists}
          selectedId={selectedId}
          onSelect={setSelectedId}
          servers={servers}
          onCreated={handleCreated}
        />

        <div style={{ flex: 1 }}>
          {selectedPlaylist ? (
            <PlaylistEditor
              playlist={selectedPlaylist}
              series={series}
              servers={servers}
              onUpdate={handlePlaylistUpdate}
              onStatus={showStatus}
            />
          ) : (
            <div className="empty-state">
              Select or create a playlist to begin editing.
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function PlaylistSidebar({
  playlists, selectedId, onSelect, servers, onCreated,
}: {
  playlists: Playlist[]
  selectedId: string | null
  onSelect: (id: string) => void
  servers: MediaServer[]
  onCreated: () => void
}) {
  const [creating, setCreating] = useState(false)
  const [name, setName] = useState('')
  const [serverId, setServerId] = useState(servers[0]?.id || '')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (servers.length > 0 && !serverId) {
      setServerId(servers[0].id)
    }
  }, [servers, serverId])

  const createPlaylist = async () => {
    if (!name || !serverId) return
    setSaving(true)
    try {
      await api.playlists.create({ media_server_id: serverId, name })
      setName('')
      setCreating(false)
      onCreated()
    } catch (e: any) {
      console.error(e)
    }
    setSaving(false)
  }

  return (
    <aside className="playlist-sidebar">
      <div className="section-label">Playlists</div>
      <div className="playlist-list">
        {playlists.map(p => (
          <div
            key={p.id}
            onClick={() => onSelect(p.id)}
            className={`playlist-choice playlist-${playlistHealth(p)} ${selectedId === p.id ? 'selected' : ''}`}
          >
            <div>{p.name}</div>
              <div className="playlist-meta">
                {p.queue_pending_count} queued <span className={p.enabled ? 'enabled' : ''}>
                {p.enabled ? 'enabled' : 'disabled'}
              </span>
            </div>
          </div>
        ))}
      </div>

      {creating ? (
        <div className="create-playlist">
          <input
            type="text"
            placeholder="Playlist name"
            value={name}
            onChange={e => setName(e.target.value)}
            style={{ width: '100%', padding: '0.3rem', marginBottom: '0.3rem', borderRadius: 4, border: '1px solid #ccc', boxSizing: 'border-box' }}
          />
          <select
            value={serverId}
            onChange={e => setServerId(e.target.value)}
            style={{ width: '100%', padding: '0.3rem', marginBottom: '0.3rem', borderRadius: 4, border: '1px solid #ccc', boxSizing: 'border-box' }}
          >
            {servers.map(s => (
              <option key={s.id} value={s.id}>{s.name || s.url}</option>
            ))}
          </select>
          <div style={{ display: 'flex', gap: '0.3rem' }}>
            <button onClick={createPlaylist} disabled={saving || !name || !serverId} style={smallBtn}>
              {saving ? 'Creating...' : 'Create'}
            </button>
            <button onClick={() => setCreating(false)} style={smallBtn}>Cancel</button>
          </div>
        </div>
      ) : (
        <button onClick={() => setCreating(true)} className="button primary sidebar-action">
          + Create Playlist
        </button>
      )}
      {selectedId && (
        <button
          onClick={async () => {
            if (!confirm('Delete this playlist?')) return
            try {
              await api.playlists.delete(selectedId)
              onSelect(playlists.filter(p => p.id !== selectedId)[0]?.id || '')
              onCreated()
            } catch (e: any) {
              console.error(e)
            }
          }}
          className="button danger sidebar-action"
        >
          Delete Playlist
        </button>
      )}
    </aside>
  )
}

function playlistHealth(playlist: Playlist) {
  if (!playlist.enabled) return 'paused'
  if (playlist.queue_pending_count >= playlist.queue_target_count) return 'healthy'
  if (playlist.queue_pending_count > 0) return 'low'
  return 'empty'
}

function PlaylistEditor({
  playlist, series, servers, onUpdate, onStatus,
}: {
  playlist: Playlist
  series: Series[]
  servers: MediaServer[]
  onUpdate: (updatedPlaylist?: Playlist) => void
  onStatus: (msg: string) => void
}) {
  const [detail, setDetail] = useState<Playlist | null>(null)
  const [name, setName] = useState(playlist.name)
  const [plexName, setPlexName] = useState(playlist.plex_playlist_name)
  const [targetCount, setTargetCount] = useState(playlist.queue_target_count)
  const [enabled, setEnabled] = useState(playlist.enabled)
  const [seriesSearch, setSeriesSearch] = useState('')
  const [seriesMenuOpen, setSeriesMenuOpen] = useState(false)
  const [addingSeriesID, setAddingSeriesID] = useState<string | null>(null)
  const [memberMap, setMemberMap] = useState<Record<string, PlaylistSeries>>({})
  const [slots, setSlots] = useState<PlaylistSlot[]>([])
  const [saving, setSaving] = useState(false)
  const [filling, setFilling] = useState(false)
  const [clearing, setClearing] = useState(false)
  const [refilling, setRefilling] = useState(false)
  const [publishing, setPublishing] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [settingNextFor, setSettingNextFor] = useState<string | null>(null)
  const [episodeCache, setEpisodeCache] = useState<Record<string, Episode[]>>({})
  const [nextEpisodePick, setNextEpisodePick] = useState<string>('')
  const [savingNext, setSavingNext] = useState(false)
  const [plexItems, setPlexItems] = useState<PlexPlaylistItem[]>([])
  const [plexLoaded, setPlexLoaded] = useState(false)
  const [plexLoading, setPlexLoading] = useState(false)
  const [plexSaving, setPlexSaving] = useState(false)
  const [plexSeriesID, setPlexSeriesID] = useState('')
  const [plexEpisodeID, setPlexEpisodeID] = useState('')
  const [profileWorkbenchFor, setProfileWorkbenchFor] = useState<string | null>(null)

  useEffect(() => {
    loadDetail()
    loadPlexItems()
  }, [playlist.id])

  useEffect(() => {
    if (detail) {
      setName(detail.name)
      setPlexName(detail.plex_playlist_name)
      setTargetCount(detail.queue_target_count)
      setEnabled(detail.enabled)
      const mm: Record<string, PlaylistSeries> = {}
      for (const s of detail.series || []) {
        mm[s.series_id] = s
      }
      setMemberMap(mm)
      setSlots(detail.slots || [])
    }
  }, [detail])

  const loadDetail = async () => {
    try {
      const d = await api.playlists.get(playlist.id)
      setDetail(d)
    } catch (e: any) {
      console.error(e)
    }
  }

  const loadPlexItems = async () => {
    setPlexLoading(true)
    try {
      const state = await api.playlists.plexItems(playlist.id)
      setPlexItems(state.items)
      setPlexLoaded(true)
    } catch (e: any) {
      setPlexItems([])
      setPlexLoaded(false)
      if (!String(e.message).includes('has not been published')) {
        onStatus('Failed to load Plex playlist: ' + e.message)
      }
    }
    setPlexLoading(false)
  }

  const movePlexItem = (index: number, direction: -1 | 1) => {
    const next = index + direction
    if (next < 0 || next >= plexItems.length) return
    const updated = [...plexItems]
    ;[updated[index], updated[next]] = [updated[next], updated[index]]
    setPlexItems(updated)
  }

  const removePlexItem = (index: number) => {
    setPlexItems(plexItems.filter((_, itemIndex) => itemIndex !== index))
  }

  const selectPlexSeries = async (seriesID: string) => {
    setPlexSeriesID(seriesID)
    setPlexEpisodeID('')
    if (seriesID && !episodeCache[seriesID]) {
      try {
        const res = await api.playlists.listEpisodes(playlist.id, seriesID)
        setEpisodeCache(prev => ({ ...prev, [seriesID]: res.episodes }))
      } catch (e: any) {
        onStatus('Failed to load episodes: ' + e.message)
      }
    }
  }

  const addPlexItem = () => {
    const episode = (episodeCache[plexSeriesID] || []).find(ep => ep.id === plexEpisodeID)
    const show = (detail?.series || []).find(item => item.series_id === plexSeriesID)
    if (!episode || !show) return
    setPlexItems([...plexItems, {
      server_episode_id: episode.server_episode_id,
      series_title: show.title,
      episode_title: episode.title,
      season_number: episode.season_number,
      episode_number: episode.episode_number,
    }])
    setPlexEpisodeID('')
  }

  const savePlexItems = async () => {
    if (plexItems.length === 0) {
      onStatus('A Plex playlist must contain at least one episode')
      return
    }
    setPlexSaving(true)
    try {
      const state = await api.playlists.replacePlexItems(playlist.id, plexItems.map(item => item.server_episode_id))
      setPlexItems(state.items)
      setPlexLoaded(true)
      onStatus('Plex playlist updated')
    } catch (e: any) {
      onStatus('Plex playlist save failed: ' + e.message)
    }
    setPlexSaving(false)
  }

  const save = async () => {
    setSaving(true)
    try {
      await api.playlists.update(playlist.id, { name, plex_playlist_name: plexName, queue_target_count: targetCount, enabled })
      onUpdate()
      onStatus('Playlist saved')
    } catch (e: any) {
      onStatus('Save failed: ' + e.message)
    }
    setSaving(false)
  }

  const toggleSeries = async (seriesId: string, add: boolean) => {
    const current = detail?.series || []
    const updated = add
	  ? [...current.map(s => ({ series_id: s.series_id, mode: s.mode, random_episode_cooldown: s.random_episode_cooldown, show_profile_id: s.show_profile_id })), { series_id: seriesId, mode: 'serial' as const, random_episode_cooldown: 10 }]
	  : current.filter(s => s.series_id !== seriesId).map(s => ({ series_id: s.series_id, mode: s.mode, random_episode_cooldown: s.random_episode_cooldown, show_profile_id: s.show_profile_id }))

    try {
      if (add) setAddingSeriesID(seriesId)
      const updatedPlaylist = await api.playlists.setSeries(playlist.id, updated)
      setDetail(updatedPlaylist)
      onUpdate(updatedPlaylist)
      if (add) {
        setSeriesSearch('')
        setSeriesMenuOpen(false)
        onStatus('Series added to playlist')
      }
    } catch (e: any) {
      onStatus('Series update failed: ' + e.message)
    } finally {
      if (add) setAddingSeriesID(null)
    }
  }

  const setSeriesMode = async (seriesId: string, mode: 'serial' | 'non_serial') => {
    const current = detail?.series || []
      const updated = current.map(s => ({
        series_id: s.series_id,
        mode: s.series_id === seriesId ? mode : s.mode,
		random_episode_cooldown: s.random_episode_cooldown,
		show_profile_id: s.show_profile_id,
      }))
    try {
      const updatedPlaylist = await api.playlists.setSeries(playlist.id, updated)
      console.log(updatedPlaylist)
      setDetail(updatedPlaylist)
      onStatus(mode === 'non_serial' ? 'Random episode order enabled' : 'Serial episode order enabled')
      try {
        await api.playlists.fill(playlist.id)
        await loadDetail()
      } catch (e: any) {
        onStatus('Mode updated, but queue refill failed: ' + e.message)
      }
    } catch (e: any) {
      onStatus('Mode update failed: ' + e.message)
    }
  }

	const setRandomEpisodeCooldown = async (seriesId: string, cooldown: number) => {
		const updated = (detail?.series || []).map(s => ({
			series_id: s.series_id,
			mode: s.mode,
			random_episode_cooldown: s.series_id === seriesId ? cooldown : s.random_episode_cooldown,
			show_profile_id: s.show_profile_id,
		}))
		try {
			const updatedPlaylist = await api.playlists.setSeries(playlist.id, updated)
			setDetail(updatedPlaylist)
			onUpdate(updatedPlaylist)
			onStatus(`Random episode cooldown set to ${cooldown} plays`)
		} catch (e: any) {
			onStatus('Cooldown update failed: ' + e.message)
		}
	}

  const assignProfile = async (seriesId: string, profileId: string) => {
    const updated = (detail?.series || []).map(s => ({ series_id: s.series_id, mode: s.mode, show_profile_id: s.series_id === seriesId ? profileId : s.show_profile_id }))
    const result = await api.playlists.setSeries(playlist.id, updated)
    setDetail(result)
    onUpdate(result)
  }

  const setNextEpisode = async (seriesID: string, episodeID: string) => {
    if (!episodeID) return
    setNextEpisodePick(episodeID)
    setSavingNext(true)
    try {
      await api.playlists.setCursor(playlist.id, seriesID, episodeID)
      await loadDetail()
      onUpdate()
      setSettingNextFor(null)
      onStatus('Next episode updated')
    } catch (e: any) {
      onStatus('Set next failed: ' + e.message)
    }
    setSavingNext(false)
  }

  const updateSlot = (idx: number, slotType: string) => {
    const updated = slots.map((s, i) => i === idx ? { ...s, slot_type: slotType as PlaylistSlot['slot_type'] } : s)
    setSlots(updated)
  }

  const addSlot = () => {
    setSlots([...slots, { id: '', playlist_id: playlist.id, position: slots.length, slot_type: 'any' }])
  }

  const removeSlot = (idx: number) => {
    setSlots(slots.filter((_, i) => i !== idx).map((s, i) => ({ ...s, position: i })))
  }

  const saveSlots = async () => {
    try {
      await api.playlists.setSlots(playlist.id, slots.map(s => ({ slot_type: s.slot_type })))
      await loadDetail()
      onStatus('Slots saved')
    } catch (e: any) {
      onStatus('Slots save failed: ' + e.message)
    }
  }

  const fill = async () => {
    setFilling(true)
    try {
      const res = await api.playlists.fill(playlist.id)
      await loadDetail()
      const q = res.queued ?? 0
      if (q > 0) {
        onStatus(`Queued ${q} episode(s)`)
      } else {
        onStatus('No eligible episodes to queue — add series with imported episodes first')
      }
    } catch (e: any) {
      onStatus('Fill failed: ' + e.message)
    }
    setFilling(false)
  }

  const clear = async () => {
    if (!confirm('Clear all queued episodes from this queue and Plex playlist?')) return
    setClearing(true)
    try {
      await api.playlists.clear(playlist.id)
      await loadDetail()
      setPlexItems([])
      setPlexLoaded(true)
      onStatus('Queue cleared')
    } catch (e: any) {
      onStatus('Clear failed: ' + e.message)
    }
    setClearing(false)
  }

  const refill = async () => {
    if (!confirm('Replace the current queue with a newly generated queue?')) return
    setRefilling(true)
    try {
      const res = await api.playlists.refill(playlist.id)
      await loadDetail()
      await loadPlexItems()
      onStatus(res.queued > 0 ? `Rebuilt and published ${res.queued} episode(s)` : 'Queue cleared; no eligible episodes were available')
    } catch (e: any) {
      onStatus('Refill failed: ' + e.message)
    }
    setRefilling(false)
  }

  const publish = async () => {
    setPublishing(true)
    try {
      const res = await api.playlists.publish(playlist.id)
      await loadDetail()
      await loadPlexItems()
      if (res.status === 'published') {
        onStatus('Published to Plex')
      } else {
        onStatus('Publish completed')
      }
    } catch (e: any) {
      onStatus('Publish failed: ' + e.message)
    }
    setPublishing(false)
  }

  const sync = async () => {
    setSyncing(true)
    try {
      const res = await api.playlists.sync(playlist.id)
      await loadDetail()
      const w = (res as any).watched ?? 0
      const q = (res as any).queued ?? 0
      const parts: string[] = []
      if (w > 0) parts.push(`Synced ${w} watched episode(s)`)
      if (q > 0) parts.push(`Queued ${q} new episode(s)`)
      onStatus(parts.length > 0 ? parts.join(', ') : 'No newly watched episodes found')
    } catch (e: any) {
      onStatus('Sync failed: ' + e.message)
    }
    setSyncing(false)
  }

  const attachedSeries = (detail?.series || []).filter(s => memberMap[s.series_id])
  const attachedIds = new Set(attachedSeries.map(s => s.series_id))

  const availableSeries = series.filter(s =>
    s.media_server_id === playlist.media_server_id &&
    !attachedIds.has(s.id) &&
    s.title.toLowerCase().includes(seriesSearch.toLowerCase())
  )

  const serverName = servers.find(s => s.id === playlist.media_server_id)?.name || playlist.media_server_id

  if (!detail) return <div className="empty-state">Loading playlist workspace...</div>

  return (
    <div className="playlist-workspace">
      <section className="playlist-overview">
        <div>
          <p className="eyebrow">PLAYLIST WORKSPACE</p>
          <h2>{detail.name}</h2>
          <p className="overview-meta">{detail.queue_pending_count} episodes ready <span>·</span> {formatDuration(detail.remaining_serial_duration_seconds)} remaining in serial shows <span>·</span> target {detail.queue_target_count} <span>·</span> {enabled ? 'live' : 'paused'}</p>
        </div>
        <div className="playlist-actions">
          <button onClick={fill} disabled={filling} className="button primary">{filling ? 'Filling...' : 'Fill queue'}</button>
          <button onClick={sync} disabled={syncing} className="button accent">{syncing ? 'Syncing...' : 'Sync Plex'}</button>
          <button onClick={refill} disabled={refilling} className="button">{refilling ? 'Refilling...' : 'Rebuild'}</button>
        </div>
      </section>

      <details className="settings-panel">
        <summary>Playlist settings and queue controls</summary>
      <div className="settings-grid">
        <label>
          Name:
          <input type="text" value={name} onChange={e => setName(e.target.value)}
            style={{ display: 'block', width: '100%', padding: '0.3rem', borderRadius: 4, border: '1px solid #ccc', boxSizing: 'border-box' }} />
        </label>
        <label>
          Plex Playlist Name:
          <input type="text" value={plexName} onChange={e => setPlexName(e.target.value)}
            style={{ display: 'block', width: '100%', padding: '0.3rem', borderRadius: 4, border: '1px solid #ccc', boxSizing: 'border-box' }} />
        </label>
        <label>
          Queue Target:
          <input type="number" min={1} value={targetCount} onChange={e => setTargetCount(+e.target.value)}
            style={{ display: 'block', width: '100%', padding: '0.3rem', borderRadius: 4, border: '1px solid #ccc', boxSizing: 'border-box' }} />
        </label>
        <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginTop: '1.2rem' }}>
          <input type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} />
          Enabled
        </label>
        <div style={{ color: '#666', fontSize: '0.9rem' }}>
          Server: <strong>{serverName}</strong>
          <br />
          Cursor: {detail.cycle_cursor} | Pending: {detail.queue_pending_count}
        </div>
      </div>

      <div className="settings-actions">
        <button onClick={save} disabled={saving} style={btnStyle}>{saving ? 'Saving...' : 'Save'}</button>
        <button onClick={fill} disabled={filling} style={{ ...btnStyle, background: '#0066cc', color: 'white', border: 'none' }}>
          {filling ? 'Filling...' : 'Fill'}
        </button>
        <button onClick={clear} disabled={clearing} style={{ ...btnStyle, background: '#9f3131', color: 'white', border: 'none' }}>
          {clearing ? 'Clearing...' : 'Clear Queue'}
        </button>
        <button onClick={refill} disabled={refilling} style={{ ...btnStyle, background: '#6b4db6', color: 'white', border: 'none' }}>
          {refilling ? 'Refilling...' : 'Refill Queue'}
        </button>
        <button onClick={publish} disabled={publishing} style={{ ...btnStyle, background: '#28a745', color: 'white', border: 'none' }}>
          {publishing ? 'Publishing...' : 'Publish'}
        </button>
        <button onClick={sync} disabled={syncing} style={{ ...btnStyle, background: '#ffc107', border: 'none' }}>
          {syncing ? 'Syncing...' : 'Sync'}
        </button>
      </div>
      </details>

      <div className="workspace-columns">
      <section className="series-panel panel">
      <div className="panel-heading"><div><p className="eyebrow">WATCHLIST</p><h3>Series runway</h3></div><strong>{attachedSeries.length}</strong></div>
      <div className="series-list">
        {attachedSeries.length === 0 && (
          <div style={{ padding: '1rem', color: '#888', textAlign: 'center' }}>
            No series attached. Use the search below to add series.
          </div>
        )}
        {attachedSeries.map(s => {
          const ps = memberMap[s.series_id]
          const isSetting = settingNextFor === s.series_id
          const episodes = episodeCache[s.series_id] || []
          const selectedQueueItem = (detail.queue_items || []).find(item =>
            item.series_id === s.series_id &&
            (item.status === 'pending' || item.status === 'pushed' || item.status === 'watching')
          )
          const watchedEpisodes = ps.mode === 'serial' && ps.next_position != null
            ? Math.min(Math.max(ps.next_position - 1, 0), ps.total_episodes)
            : ps.watched_episodes
          const progressPct = ps.total_episodes > 0
            ? watchedEpisodes / ps.total_episodes * 100
            : 0
          const remainingPct = 100 - progressPct
          return (
            <React.Fragment key={s.id}>
              <div className="series-card" style={{ '--progress': `${remainingPct}%`, '--runway-color': `hsl(${Math.max(0, Math.min(120, remainingPct * 1.2))} 63% 42%)` } as React.CSSProperties}>
                <div className="series-card-header">
                  <div className="series-title">{s.title}</div>
                  <div className="progress-stat"><strong>{progressPct.toFixed(0)}%</strong><span>complete</span></div>
                </div>
                <button
                  className="series-mode-toggle"
                  onClick={() => setSeriesMode(s.series_id, ps.mode === 'serial' ? 'non_serial' : 'serial')}
                  title={ps.mode === 'serial' ? 'Switch to random episode order' : 'Switch to serial episode order'}
                  aria-label={ps.mode === 'serial' ? 'Switch to random episode order' : 'Switch to serial episode order'}
                >
                  {ps.mode === 'serial' ? 'S' : 'R'}
                </button>
                <div className="series-details">
                  <div className="series-next">
                    {ps.mode === 'serial' ? (
                      ps.next_episode_title ? (
                        <>Next: S{String(ps.next_season_number || 0).padStart(2, '0')}E{String(ps.next_episode_number || 0).padStart(2, '0')} - {ps.next_episode_title}</>
                      ) : ps.total_episodes > 0 ? (
                        <span style={{ color: '#888' }}>Completed</span>
                      ) : (
                        <span style={{ color: '#c00' }}>No episodes imported</span>
                      )
                    ) : (
                      selectedQueueItem ? (
                        <>Selected: S{String(selectedQueueItem.season_number || 0).padStart(2, '0')}E{String(selectedQueueItem.episode_number || 0).padStart(2, '0')} - {selectedQueueItem.episode_title}</>
                      ) : (
                        <span style={{ color: '#666' }}>Random episode order</span>
                      )
                    )}
                    <span className="series-progress-copy">{watchedEpisodes}/{ps.total_episodes} watched · {Math.max(ps.total_episodes - watchedEpisodes, 0)} left · {ps.last_seen_at ? `Last seen ${new Date(ps.last_seen_at).toLocaleDateString()}` : 'Never seen'}</span>
                  </div>
                </div>
                <div className="series-card-actions">
                  {ps.mode === 'serial' && ps.total_episodes > 0 && (
                    <button
                      onClick={async () => {
                        setSettingNextFor(s.series_id)
                        setNextEpisodePick(ps.next_episode_id || '')
                        if (!episodeCache[s.series_id]) {
                          try {
                            const res = await api.playlists.listEpisodes(playlist.id, s.series_id)
                            setEpisodeCache(prev => ({ ...prev, [s.series_id]: res.episodes }))
                          } catch (e: any) {
                            onStatus('Failed to load episodes: ' + e.message)
                          }
                        }
                      }}
                      style={{ ...smallBtn, fontSize: '0.8rem' }}
                      title="Set next episode"
                    >
                      Set Next
                    </button>
                  )}
                  {ps.mode === 'non_serial' && (
                    <label title="Episodes return to the random pool after this many other completed plays">
                      Cooldown
                      <input
                        type="number"
                        min={0}
                        value={ps.random_episode_cooldown}
                        onChange={e => setRandomEpisodeCooldown(s.series_id, Math.max(0, Number(e.target.value) || 0))}
                        style={{ width: '3.5rem', marginLeft: '0.25rem', padding: '0.2rem', borderRadius: 4, border: '1px solid #ccc', fontSize: '0.85rem' }}
                      />
                    </label>
                  )}
                  <button onClick={() => setProfileWorkbenchFor(s.series_id)} style={smallBtn}>Profile</button>
                  <button
                    onClick={() => toggleSeries(s.series_id, false)}
                    className="icon-button remove-button"
                    title="Remove from playlist"
                  >
                    Remove
                  </button>
                </div>
              </div>
              {isSetting && (
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', padding: '0.5rem 0.5rem 0.5rem 1rem', borderBottom: '1px solid #eee', background: '#fafafa' }}>
                    <select
                      value={nextEpisodePick}
                      onChange={e => setNextEpisode(s.series_id, e.target.value)}
                      disabled={savingNext}
                      style={{ flex: 1, minWidth: 0, padding: '0.3rem', borderRadius: 4, border: '1px solid #ccc', fontSize: '0.85rem' }}
                    >
                    <option value="">Select episode...</option>
                    {episodes.map(ep => (
                      <option key={ep.id} value={ep.id}>
                        S{String(ep.season_number).padStart(2, '0')}E{String(ep.episode_number).padStart(2, '0')} - {ep.title} ({ep.rating > 0 ? `Rating ${ep.rating.toFixed(1)}` : 'Rating unavailable'})
                      </option>
                    ))}
                    </select>
                  {savingNext && <span className="inline-status">Saving...</span>}
                  <button
                    onClick={() => setSettingNextFor(null)}
                    style={smallBtn}
                  >
                    Cancel
                  </button>
                </div>
              )}
            </React.Fragment>
          )
        })}
      </div>

      <div className="add-series">
      <h3>Add series</h3>
        <input
          type="text"
          placeholder="Search series from this media server..."
          value={seriesSearch}
          onChange={e => {
            setSeriesSearch(e.target.value)
            setSeriesMenuOpen(true)
          }}
          onFocus={() => setSeriesMenuOpen(true)}
          onBlur={() => setTimeout(() => setSeriesMenuOpen(false), 150)}
          aria-expanded={seriesMenuOpen}
          aria-controls="available-series-menu"
          style={{ width: '100%', padding: '0.4rem', marginBottom: '0.5rem', borderRadius: 4, border: '1px solid #ccc', boxSizing: 'border-box' }}
        />
       {seriesMenuOpen && (
         <div id="available-series-menu" className="series-menu">
           {availableSeries.length === 0 ? (
             <div className="series-menu-empty">No matching series found</div>
           ) : (
              availableSeries.map(s => (
                <button
                  type="button"
                  key={s.id}
                  disabled={addingSeriesID !== null}
                  onMouseDown={e => e.preventDefault()}
                  onClick={() => {
                    toggleSeries(s.id, true)
                  }}
                  className="series-menu-option"
                >
                  <span>+</span>
                  <span>{addingSeriesID === s.id ? 'Adding...' : s.title}</span>
                </button>
              ))
           )}
         </div>
      )}
      </div>

      <details className="slot-settings">
      <summary>Rotation slots ({slots.length})</summary>
      <div style={{ marginBottom: '0.5rem' }}>
        {slots.map((slot, i) => (
          <div key={i} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.3rem' }}>
            <span style={{ fontWeight: 600, minWidth: '1.5rem' }}>{i + 1}.</span>
            <select
              value={slot.slot_type}
              onChange={e => updateSlot(i, e.target.value)}
              style={{ flex: 1, padding: '0.3rem', borderRadius: 4, border: '1px solid #ccc' }}
            >
              <option value="top_rated">Top Rated (Random)</option>
              <option value="any">Any Episode</option>
              <option value="lowest_rated">Lowest Rated</option>
              <option value="least_recently_seen">Least Recently Seen</option>
            </select>
            <button onClick={() => removeSlot(i)} style={{ ...smallBtn, color: '#c00' }}>x</button>
          </div>
        ))}
        <button onClick={addSlot} style={smallBtn}>+ Add Slot</button>
        <button onClick={saveSlots} style={{ ...smallBtn, marginLeft: '0.5rem' }}>Save Slots</button>
      </div>
      </details>
      </section>

      <section className="queue-panel panel">
      <div className="panel-heading"><div><p className="eyebrow">PLEX PLAYLIST</p><h3>Queue runway</h3></div><strong>{detail.queue_pending_count} ready</strong></div>
      <div className="queue-table-wrap">
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.9rem' }}>
          <thead>
            <tr style={{ background: '#f5f5f5' }}>
              <th style={thStyle}>#</th>
              <th style={thStyle}>Slot</th>
              <th style={thStyle}>Series</th>
              <th style={thStyle}>Episode</th>
              <th style={thStyle}>Rating</th>
              <th style={thStyle}>Score</th>
              <th style={thStyle}>Status</th>
            </tr>
          </thead>
          <tbody>
            {(detail.queue_items || []).map(qi => (
              <tr key={qi.id}>
                <td style={tdStyle}>{qi.position}</td>
                <td style={tdStyle}>
                  <span className="slot-badge">
                    {qi.slot_type.replace('_', ' ')}
                  </span>
                </td>
                <td style={tdStyle}>{qi.series_title || qi.series_id.slice(0, 8)}</td>
                <td style={tdStyle}>
                  S{String(qi.season_number || 0).padStart(2, '0')}E{String(qi.episode_number || 0).padStart(2, '0')}
                  {qi.episode_title ? ` - ${qi.episode_title}` : ''}
                </td>
                <td style={tdStyle}>{qi.episode_rating != null ? qi.episode_rating.toFixed(1) : 'Unavailable'}</td>
                <td style={tdStyle}>{qi.score != null ? qi.score.toFixed(2) : '—'}</td>
                <td style={tdStyle}>
                  <span style={{
                    color: qi.status === 'watched' ? '#080' :
                           qi.status === 'watching' ? '#d69e2e' :
                           qi.status === 'pushed' ? '#0066cc' :
                           qi.status === 'skipped' ? '#888' : '#666'
                  }}>
                    {qi.status}
                  </span>
                </td>
              </tr>
            ))}
            {(!detail.queue_items || detail.queue_items.length === 0) && (
              <tr><td colSpan={7} style={{ padding: '1rem', textAlign: 'center', color: '#888' }}>Queue is empty. Click Fill to populate.</td></tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="plex-panel-header"><h3>Plex order <span>{plexLoaded ? plexItems.length : 'not published'}</span></h3><p>Changes replace the local Plex playlist.</p></div>
      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.5rem' }}>
        <button onClick={loadPlexItems} disabled={plexLoading} style={smallBtn}>
          {plexLoading ? 'Refreshing...' : 'Refresh Plex'}
        </button>
        <button onClick={savePlexItems} disabled={!plexLoaded || plexSaving || plexItems.length === 0} style={{ ...smallBtn, background: '#28a745', color: 'white', border: 'none' }}>
          {plexSaving ? 'Saving...' : 'Save Plex Order'}
        </button>
      </div>
      {plexLoaded && (
        <>
          <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.5rem' }}>
            <select value={plexSeriesID} onChange={e => selectPlexSeries(e.target.value)} style={{ flex: 1, padding: '0.3rem' }}>
              <option value="">Add from attached series...</option>
              {attachedSeries.map(item => <option key={item.id} value={item.series_id}>{item.title}</option>)}
            </select>
            <select value={plexEpisodeID} onChange={e => setPlexEpisodeID(e.target.value)} disabled={!plexSeriesID} style={{ flex: 2, padding: '0.3rem' }}>
              <option value="">Select episode...</option>
              {(episodeCache[plexSeriesID] || []).map(ep => <option key={ep.id} value={ep.id}>S{String(ep.season_number).padStart(2, '0')}E{String(ep.episode_number).padStart(2, '0')} - {ep.title}</option>)}
            </select>
            <button onClick={addPlexItem} disabled={!plexEpisodeID} style={smallBtn}>Add</button>
          </div>
          <div style={{ border: '1px solid #ddd', borderRadius: 4 }}>
            {plexItems.map((item, index) => (
              <div key={`${item.server_episode_id}-${index}`} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', padding: '0.4rem', borderBottom: index + 1 < plexItems.length ? '1px solid #eee' : 'none' }}>
                <span style={{ minWidth: '1.5rem' }}>{index + 1}.</span>
                <span style={{ flex: 1 }}>{item.series_title} - S{String(item.season_number).padStart(2, '0')}E{String(item.episode_number).padStart(2, '0')} {item.episode_title}</span>
                <button onClick={() => movePlexItem(index, -1)} disabled={index === 0} style={smallBtn}>Up</button>
                <button onClick={() => movePlexItem(index, 1)} disabled={index + 1 === plexItems.length} style={smallBtn}>Down</button>
                <button onClick={() => removePlexItem(index)} style={{ ...smallBtn, color: '#c00' }}>Remove</button>
              </div>
            ))}
          </div>
        </>
      )}
      </section>
      </div>
      {profileWorkbenchFor && (() => {
        const member = (detail.series || []).find(s => s.series_id === profileWorkbenchFor)
        return member ? <ShowProfileWorkbench playlistId={playlist.id} seriesId={member.series_id} title={member.title} assignedProfileId={member.show_profile_id} onAssign={(profileId) => assignProfile(member.series_id, profileId)} onClose={() => { setProfileWorkbenchFor(null); loadDetail() }} onStatus={onStatus} /> : null
      })()}
    </div>
  )
}

const thStyle: React.CSSProperties = {
  padding: '0.4rem',
  textAlign: 'left',
  borderBottom: '2px solid #ddd',
  position: 'sticky',
  top: 0,
  background: '#f5f5f5',
}

const tdStyle: React.CSSProperties = {
  padding: '0.4rem',
  borderBottom: '1px solid #eee',
}

const btnStyle: React.CSSProperties = {
  padding: '0.4rem 0.8rem',
  borderRadius: 4,
  cursor: 'pointer',
  border: '1px solid #ccc',
  background: '#f8f8f8',
}

const smallBtn: React.CSSProperties = {
  padding: '0.3rem 0.6rem',
  borderRadius: 4,
  cursor: 'pointer',
  border: '1px solid #ccc',
  background: '#f8f8f8',
  fontSize: '0.85rem',
}

function formatDuration(seconds: number) {
  const totalMinutes = Math.floor(seconds / 60)
  const days = Math.floor(totalMinutes / (24 * 60))
  const hours = Math.floor(totalMinutes % (24 * 60) / 60)
  const minutes = totalMinutes % 60
  const parts: string[] = []

  if (days > 0) parts.push(`${days}d`)
  if (hours > 0) parts.push(`${hours}h`)
  if (minutes > 0 || parts.length === 0) parts.push(`${minutes}m`)

  return parts.join(' ')
}
