import React, { Component } from 'react'

interface Props {
  children: React.ReactNode
}

interface State {
  error: Error | null
}

export default class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error) {
    return { error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error('React error:', error, info)
  }

  render() {
    if (this.state.error) {
      return (
        <div style={{ maxWidth: 600, margin: '2rem auto', padding: '1rem', fontFamily: 'system-ui, sans-serif' }}>
          <h2 style={{ color: '#c00' }}>Something went wrong</h2>
          <pre style={{ background: '#fee', padding: '1rem', borderRadius: 4, overflow: 'auto', fontSize: '0.85rem' }}>
            {this.state.error.message}
          </pre>
          <button
            onClick={() => { this.setState({ error: null }); window.location.reload() }}
            style={{ padding: '0.5rem 1rem', marginTop: '1rem', cursor: 'pointer' }}
          >
            Reload
          </button>
        </div>
      )
    }
    return this.props.children
  }
}