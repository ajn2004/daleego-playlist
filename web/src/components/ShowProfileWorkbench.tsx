import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Episode, ShowProfile, ShowProfileDetail, ShowProfileRule } from '../types'

type Props = {
  playlistId: string
  seriesId: string
  title: string
  assignedProfileId?: string | null
  onAssign: (profileId: string) => Promise<void>
  onClose: () => void
  onStatus: (message: string) => void
}

export function ShowProfileWorkbench({ playlistId, seriesId, title, assignedProfileId, onAssign, onClose, onStatus }: Props) {
  const [profiles, setProfiles] = useState<ShowProfile[]>([])
  const [detail, setDetail] = useState<ShowProfileDetail | null>(null)
  const [episodes, setEpisodes] = useState<Episode[]>([])
  const [search, setSearch] = useState('')
  const [saving, setSaving] = useState(false)

  const load = async (profileId?: string) => {
    const [profileResult, episodeResult] = await Promise.all([
      api.showProfiles.list(seriesId),
      api.playlists.listEpisodes(playlistId, seriesId),
    ])
    setProfiles(profileResult.show_profiles)
    setEpisodes(episodeResult.episodes)
    const id = profileId || assignedProfileId || profileResult.show_profiles[0]?.id
    if (id) setDetail(await api.showProfiles.get(seriesId, id))
  }

  useEffect(() => { load().catch(e => onStatus(`Failed to load profiles: ${e.message}`)) }, [seriesId, playlistId])

  const updateRule = (kind: 'season' | 'episode', key: number | string, allowed?: boolean) => {
    if (!detail) return
    const field = kind === 'season' ? 'season_rules' : 'episode_rules'
    const rules = [...detail[field]]
    const index = rules.findIndex(rule => kind === 'season' ? rule.season_number === key : rule.episode_id === key)
    if (index >= 0) {
      if (allowed === undefined) rules.splice(index, 1)
      else rules[index] = { ...rules[index], allowed }
    } else if (allowed !== undefined) {
      rules.push(kind === 'season' ? { season_number: key as number, allowed } : { episode_id: key as string, allowed })
    }
    setDetail({ ...detail, [field]: rules })
  }

  const save = async () => {
    if (!detail) return
    setSaving(true)
    try {
      const updated = await api.showProfiles.update(seriesId, detail.id, {
        name: detail.name,
        default_mode: detail.default_mode,
        season_rules: detail.season_rules,
        episode_rules: detail.episode_rules,
      })
      setDetail(updated)
      await load(updated.id)
      onStatus('Profile saved. Excluded queued episodes were skipped; refill to replace them.')
    } catch (e: any) { onStatus(`Profile save failed: ${e.message}`) }
    setSaving(false)
  }

  const create = async () => {
    const name = prompt('Profile name')?.trim()
    if (!name) return
    try {
      const created = await api.showProfiles.create(seriesId, { name, default_mode: 'allow' })
      await load(created.id)
    } catch (e: any) { onStatus(`Profile creation failed: ${e.message}`) }
  }

  const selectProfile = async (id: string) => { setDetail(await api.showProfiles.get(seriesId, id)) }
  const seasons = [...new Set(episodes.map(ep => ep.season_number))].sort((a, b) => a - b)
  const visibleEpisodes = episodes.filter(ep => `${ep.season_number} ${ep.episode_number} ${ep.title}`.toLowerCase().includes(search.toLowerCase()))
  const ruleFor = (rules: ShowProfileRule[], key: number | string, kind: 'season' | 'episode') => rules.find(rule => kind === 'season' ? rule.season_number === key : rule.episode_id === key)

  return <div className="profile-modal" role="dialog" aria-modal="true" aria-label={`${title} show profiles`}>
    <div className="profile-workbench">
      <header className="profile-header"><div><p className="eyebrow">SHOW PROFILE</p><h2>{title}</h2></div><button onClick={onClose}>Close</button></header>
      {!detail ? <p>Loading profiles...</p> : <>
        <div className="profile-toolbar">
          <label>Profile <select value={detail.id} onChange={e => selectProfile(e.target.value)}>{profiles.map(profile => <option key={profile.id} value={profile.id}>{profile.name}{profile.is_default ? ' (default)' : ''}</option>)}</select></label>
          <button onClick={create}>New profile</button>
          <button onClick={async () => { await api.showProfiles.setDefault(seriesId, detail.id); await load(detail.id) }}>Make default</button>
          <button disabled={detail.is_default} onClick={async () => {
            if (!confirm(`Delete ${detail.name}? Playlist memberships using it will use the default profile.`)) return
            try {
              await api.showProfiles.delete(seriesId, detail.id)
              await load()
              onStatus('Profile deleted')
            } catch (e: any) { onStatus(`Profile deletion failed: ${e.message}`) }
          }}>Delete</button>
          <label>Playlist profile <select value={assignedProfileId || ''} onChange={e => onAssign(e.target.value)}>{profiles.map(profile => <option key={profile.id} value={profile.id}>{profile.name}</option>)}</select></label>
        </div>
        <div className="profile-settings">
          <label>Name <input value={detail.name} onChange={e => setDetail({ ...detail, name: e.target.value })} /></label>
          <label>Base rule <select value={detail.default_mode} onChange={e => setDetail({ ...detail, default_mode: e.target.value as 'allow' | 'deny' })}><option value="allow">Allow all unless excluded</option><option value="deny">Exclude all unless allowed</option></select></label>
          <strong>{detail.eligible_episodes} eligible episodes</strong>
        </div>
        <section><h3>Seasons</h3><div className="season-rules">{seasons.map(season => { const rule = ruleFor(detail.season_rules, season, 'season'); return <div key={season}>Season {season} <span>{rule ? (rule.allowed ? 'allowed' : 'excluded') : 'inherit'}</span><button onClick={() => updateRule('season', season, true)}>Allow</button><button onClick={() => updateRule('season', season, false)}>Exclude</button><button onClick={() => updateRule('season', season)}>Clear</button></div> })}</div></section>
        <section><h3>Episodes</h3><input className="profile-search" placeholder="Filter episodes" value={search} onChange={e => setSearch(e.target.value)} />
          <div className="profile-episodes">{visibleEpisodes.map(ep => { const rule = ruleFor(detail.episode_rules, ep.id, 'episode'); return <div key={ep.id}><span>S{String(ep.season_number).padStart(2, '0')}E{String(ep.episode_number).padStart(2, '0')} {ep.title}</span><small>{rule ? (rule.allowed ? 'allowed' : 'excluded') : 'inherit'}</small><button onClick={() => updateRule('episode', ep.id, true)}>Allow</button><button onClick={() => updateRule('episode', ep.id, false)}>Exclude</button><button onClick={() => updateRule('episode', ep.id)}>Clear</button></div> })}</div>
        </section>
        <footer><button className="button primary" disabled={saving} onClick={save}>{saving ? 'Saving...' : 'Save profile'}</button></footer>
      </>}
    </div>
  </div>
}
