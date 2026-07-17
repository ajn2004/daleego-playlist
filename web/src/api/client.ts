const BASE = ''

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body?.error?.message || `HTTP ${res.status}`)
  }
  return res.json()
}

export const api = {
  status: () => request<{ status: string }>('/api/v1/status'),

  series: {
    list: () => request<{ series: import('../types').Series[] }>('/api/v1/series'),
    get: (id: string) => request<import('../types').Series>(`/api/v1/series/${id}`),
    update: (id: string, data: Partial<import('../types').Series>) =>
      request<import('../types').Series>(`/api/v1/series/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    sync: (id: string) =>
      request<{ status: string }>(`/api/v1/series/${id}/sync`, { method: 'POST' }),
  },

  profiles: {
    list: () =>
      request<{ rotation_profiles: import('../types').RotationProfile[] }>('/api/v1/rotation-profiles'),
    create: (data: import('../types').Policy) =>
      request<import('../types').RotationProfile>('/api/v1/rotation-profiles', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    update: (id: string, data: import('../types').Policy) =>
      request<import('../types').RotationProfile>(`/api/v1/rotation-profiles/${id}`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
  },

  rotations: {
    current: () => request<import('../types').Rotation>('/api/v1/rotations/current'),
    generate: () =>
      request<import('../types').RotationItem[]>('/api/v1/rotations/generate', {
        method: 'POST',
        body: '{}',
      }),
    publish: (id: string) =>
      request<{ status: string }>(`/api/v1/rotations/${id}/publish`, { method: 'POST' }),
    reroll: (id: string) =>
      request<import('../types').RotationItem[]>(`/api/v1/rotations/${id}/reroll`, {
        method: 'POST',
      }),
    sync: () =>
      request<{ status: string }>('/api/v1/rotations/current/sync', { method: 'POST' }),
  },

  servers: {
    list: () => request<{ media_servers: import('../types').MediaServer[] }>('/api/v1/media-servers'),
    create: (url: string, token: string) =>
      request<import('../types').MediaServer>('/api/v1/media-servers', {
        method: 'POST',
        body: JSON.stringify({ url, token }),
      }),
    test: (id: string) =>
      request<{ status: string }>(`/api/v1/media-servers/${id}/test`, { method: 'POST' }),
  },

  playlists: {
    list: () => request<{ playlists: import('../types').Playlist[] }>('/api/v1/playlists'),
    get: (id: string) => request<import('../types').Playlist>(`/api/v1/playlists/${id}`),
    create: (data: { media_server_id: string; name: string; plex_playlist_name?: string; queue_target_count?: number }) =>
      request<import('../types').Playlist>('/api/v1/playlists', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    update: (id: string, data: Partial<import('../types').Playlist>) =>
      request<import('../types').Playlist>(`/api/v1/playlists/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    delete: (id: string) =>
      request<void>(`/api/v1/playlists/${id}`, { method: 'DELETE' }),
    setSeries: (id: string, series: { series_id: string; mode: string }[]) =>
      request<import('../types').Playlist>(`/api/v1/playlists/${id}/series`, {
        method: 'PUT',
        body: JSON.stringify({ series }),
      }),
    setSlots: (id: string, slots: { slot_type: string }[]) =>
      request<import('../types').Playlist>(`/api/v1/playlists/${id}/slots`, {
        method: 'PUT',
        body: JSON.stringify({ slots }),
      }),
    fill: (id: string) =>
      request<{ queued: number }>(`/api/v1/playlists/${id}/fill`, { method: 'POST' }),
    clear: (id: string) =>
      request<{ status: string }>(`/api/v1/playlists/${id}/clear`, { method: 'POST' }),
    refill: (id: string) =>
      request<{ status: string; queued: number }>(`/api/v1/playlists/${id}/refill`, { method: 'POST' }),
    publish: (id: string) =>
      request<{ status: string }>(`/api/v1/playlists/${id}/publish`, { method: 'POST' }),
    sync: (id: string) =>
      request<{ status: string }>(`/api/v1/playlists/${id}/sync`, { method: 'POST' }),
    plexItems: (id: string) =>
      request<import('../types').PlexPlaylistState>(`/api/v1/playlists/${id}/plex-items`),
    replacePlexItems: (id: string, serverEpisodeIds: string[]) =>
      request<import('../types').PlexPlaylistState>(`/api/v1/playlists/${id}/plex-items`, {
        method: 'PUT',
        body: JSON.stringify({ server_episode_ids: serverEpisodeIds }),
      }),
    listEpisodes: (playlistId: string, seriesId: string) =>
      request<{ episodes: import('../types').Episode[] }>(`/api/v1/playlists/${playlistId}/series/${seriesId}/episodes`),
    setCursor: (playlistId: string, seriesId: string, episodeId: string) =>
      request<import('../types').Playlist>(`/api/v1/playlists/${playlistId}/series/${seriesId}/cursor`, {
        method: 'POST',
        body: JSON.stringify({ episode_id: episodeId }),
      }),
  },
}
