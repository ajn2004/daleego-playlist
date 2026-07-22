export interface Series {
  id: string
  media_server_id: string
  server_series_id: string
  library_id: string
  title: string
  active: boolean
  created_at: string
  updated_at: string
  progress?: SeriesProgress
}

export interface SeriesProgress {
  series_id: string
  last_watched_episode_id: string | null
  next_episode_id: string | null
  next_position: number | null
  synchronized_at: string | null
  updated_at: string
}

export interface RotationProfile {
  id: string
  name: string
  enabled: boolean
  configuration: Policy
  created_at: string
  updated_at: string
}

export interface Policy {
  slots: Slot[]
  scoring: Scoring
  constraints: Constraints
}

export interface Slot {
  name: string
  rank_by: string
  direction: string
  pool_size: number
  selection: string
}

export interface Scoring {
  episode_weight: number
  arc_weight: number
  window_size: number
  window_decay: number
}

export interface Constraints {
  different_series_per_rotation: boolean
  include_specials: boolean
  session_budget_minutes: number
}

export interface Rotation {
  rotation: {
    id: string
    profile_id: string
    status: string
    generated_at: string
    published_at: string | null
    completed_at: string | null
    available_minutes: number
    random_seed: number
  }
  items: RotationItem[]
}

export interface RotationItem {
  id: string
  rotation_id: string
  position: number
  series_id: string
  episode_id: string
  slot_kind: string
  score: number
  score_details: Record<string, unknown>
  status: string
  series_title?: string
  episode_title?: string
  season_number?: number
  episode_number?: number
}

export interface MediaServer {
  id: string
  url: string
  name: string
  created_at: string
}

export interface Playlist {
  id: string
  media_server_id: string
  name: string
  plex_playlist_name: string
  queue_target_count: number
  cycle_cursor: number
  enabled: boolean
  created_at: string
  updated_at: string
  series?: PlaylistSeries[]
  slots?: PlaylistSlot[]
  queue_items?: PlaylistQueueItem[]
  queue_pending_count: number
}

export interface PlaylistSeries {
  id: string
  series_id: string
  title: string
  mode: 'serial' | 'non_serial'
  random_episode_cooldown: number
  next_position?: number | null
  next_episode_id?: string | null
  next_episode_title?: string
  next_season_number?: number
  next_episode_number?: number
  total_episodes: number
  watched_episodes: number
  progress_pct: number
  show_profile_id?: string | null
  show_profile_name?: string
  eligible_episodes: number
}

export interface ShowProfileRule {
  profile_id?: string
  season_number?: number
  episode_id?: string
  allowed: boolean
}

export interface ShowProfile {
  id: string
  series_id: string
  name: string
  default_mode: 'allow' | 'deny'
  is_default: boolean
  eligible_episodes: number
  assignments: number
  created_at: string
  updated_at: string
}

export interface ShowProfileDetail extends ShowProfile {
  season_rules: ShowProfileRule[]
  episode_rules: ShowProfileRule[]
}

export interface PlaylistSlot {
  id: string
  playlist_id: string
  position: number
  slot_type: 'top_rated' | 'any' | 'lowest_rated'
}

export interface PlaylistQueueItem {
  id: string
  position: number
  cycle_index: number
  slot_position: number
  slot_type: string
  series_id: string
  series_title: string
  episode_id: string
  episode_title: string
  season_number: number
  episode_number: number
  episode_rating: number | null
  score: number | null
  status: 'pending' | 'pushed' | 'watching' | 'watched' | 'skipped'
}

export interface PlexPlaylistItem {
  server_episode_id: string
  series_title: string
  episode_title: string
  season_number: number
  episode_number: number
}

export interface PlexPlaylistState {
  items: PlexPlaylistItem[]
}

export interface Episode {
  id: string
  series_id: string
  server_episode_id: string
  season_number: number
  episode_number: number
  absolute_order: number
  title: string
  duration: number
  rating: number
  air_date: string
  created_at: string
}
