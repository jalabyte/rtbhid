import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import Lookup from './Lookup'
import { api } from '@/lib/api'

// Mock the api module
vi.mock('@/lib/api', () => ({
  api: {
    lookup: vi.fn(),
  },
}))

describe('Lookup Page', () => {
  const mockLookup = vi.mocked(api.lookup)

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders search form with textarea and button', () => {
    render(<Lookup />)

    expect(screen.getByRole('heading', { name: /lookup/i })).toBeInTheDocument()
    expect(screen.getByPlaceholderText(/enter ip addresses or cidrs/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /search/i })).toBeInTheDocument()
  })

  it('handles valid IP input ("192.168.1.1")', async () => {
    const user = userEvent.setup()
    const mockResult = {
      results: [
        {
          query: '192.168.1.1',
          matches: [
            {
              type: 'prefix',
              source_id: 'source-1',
              source_name: 'Test Source',
              cidr: '192.168.1.0/24',
              note: 'Test note',
              created_at: '2024-01-01T00:00:00Z',
            },
          ],
        },
      ],
    }
    mockLookup.mockResolvedValueOnce(mockResult)

    render(<Lookup />)

    const textarea = screen.getByPlaceholderText(/enter ip addresses or cidrs/i)
    const button = screen.getByRole('button', { name: /search/i })

    await user.type(textarea, '192.168.1.1')
    await user.click(button)

    await waitFor(() => {
      expect(mockLookup).toHaveBeenCalledWith(['192.168.1.1'])
    })

    expect(screen.getByText('192.168.1.1')).toBeInTheDocument()
    expect(screen.getByText('Test Source')).toBeInTheDocument()
  })

  it('handles valid CIDR input ("192.168.1.0/24")', async () => {
    const user = userEvent.setup()
    const mockResult = {
      results: [
        {
          query: '192.168.1.0/24',
          matches: [
            {
              type: 'prefix',
              source_id: 'source-1',
              source_name: 'Another Source',
              cidr: '192.168.1.0/24',
              note: '',
              created_at: '2024-01-01T00:00:00Z',
            },
          ],
        },
      ],
    }
    mockLookup.mockResolvedValueOnce(mockResult)

    render(<Lookup />)

    const textarea = screen.getByPlaceholderText(/enter ip addresses or cidrs/i)
    const button = screen.getByRole('button', { name: /search/i })

    await user.type(textarea, '192.168.1.0/24')
    await user.click(button)

    await waitFor(() => {
      expect(mockLookup).toHaveBeenCalledWith(['192.168.1.0/24'])
    })

    expect(screen.getByText('192.168.1.0/24')).toBeInTheDocument()
  })

  it('handles invalid input (shows error message)', async () => {
    const user = userEvent.setup()

    render(<Lookup />)

    const textarea = screen.getByPlaceholderText(/enter ip addresses or cidrs/i)
    const button = screen.getByRole('button', { name: /search/i })

    await user.type(textarea, 'invalid-input')
    await user.click(button)

    await waitFor(() => {
      expect(screen.getByText(/invalid ip address or cidr/i)).toBeInTheDocument()
    })
  })

  it('displays results correctly (query, type, source name, CIDR)', async () => {
    const user = userEvent.setup()
    const mockResult = {
      results: [
        {
          query: '10.0.0.1',
          matches: [
            {
              type: 'prefix',
              source_id: 'source-2',
              source_name: 'Test Source',
              cidr: '10.0.0.0/8',
              note: 'Test note',
              created_at: '2024-01-01T00:00:00Z',
            },
          ],
        },
      ],
    }
    mockLookup.mockResolvedValueOnce(mockResult)

    render(<Lookup />)

    const textarea = screen.getByPlaceholderText(/enter ip addresses or cidrs/i)
    const button = screen.getByRole('button', { name: /search/i })

    await user.type(textarea, '10.0.0.1')
    await user.click(button)

    await waitFor(() => {
      expect(screen.getByText('10.0.0.1')).toBeInTheDocument()
      expect(screen.getByText('prefix')).toBeInTheDocument()
      expect(screen.getByText('Test Source')).toBeInTheDocument()
      expect(screen.getByText('10.0.0.0/8')).toBeInTheDocument()
    })
  })

  it('handles empty results ("No matches found" message)', async () => {
    const user = userEvent.setup()
    const mockResult = {
      results: [
        {
          query: '8.8.8.8',
          matches: [],
        },
      ],
    }
    mockLookup.mockResolvedValueOnce(mockResult)

    render(<Lookup />)

    const textarea = screen.getByPlaceholderText(/enter ip addresses or cidrs/i)
    const button = screen.getByRole('button', { name: /search/i })

    await user.type(textarea, '8.8.8.8')
    await user.click(button)

    await waitFor(() => {
      expect(screen.getByText(/no matches found/i)).toBeInTheDocument()
    })
  })

  it('handles bulk input (multiple lines)', async () => {
    const user = userEvent.setup()
    const mockResult = {
      results: [
        {
          query: '192.168.1.1',
          matches: [
            {
              type: 'prefix',
              source_id: 'source-1',
              source_name: 'Source 1',
              cidr: '192.168.1.0/24',
              note: '',
              created_at: '2024-01-01T00:00:00Z',
            },
          ],
        },
        {
          query: '10.0.0.1',
          matches: [
            {
              type: 'prefix',
              source_id: 'source-2',
              source_name: 'Source 2',
              cidr: '10.0.0.0/8',
              note: '',
              created_at: '2024-01-01T00:00:00Z',
            },
          ],
        },
        {
          query: '172.16.0.1',
          matches: [
            {
              type: 'prefix',
              source_id: 'source-3',
              source_name: 'Source 3',
              cidr: '172.16.0.0/12',
              note: '',
              created_at: '2024-01-01T00:00:00Z',
            },
          ],
        },
      ],
    }
    mockLookup.mockResolvedValueOnce(mockResult)

    render(<Lookup />)

    const textarea = screen.getByPlaceholderText(/enter ip addresses or cidrs/i)
    const button = screen.getByRole('button', { name: /search/i })

    await user.type(textarea, '192.168.1.1\n10.0.0.1\n172.16.0.1')
    await user.click(button)

    await waitFor(() => {
      expect(mockLookup).toHaveBeenCalledWith(['192.168.1.1', '10.0.0.1', '172.16.0.1'])
    })

    await waitFor(() => {
      expect(screen.getByText('192.168.1.1')).toBeInTheDocument()
      expect(screen.getByText('10.0.0.1')).toBeInTheDocument()
      expect(screen.getByText('172.16.0.1')).toBeInTheDocument()
    })
  })

  it('handles loading state (spinner)', async () => {
    const user = userEvent.setup()
    mockLookup.mockImplementation(() => new Promise(() => {}))

    render(<Lookup />)

    const textarea = screen.getByPlaceholderText(/enter ip addresses or cidrs/i)
    const button = screen.getByRole('button', { name: /search/i })

    await user.type(textarea, '192.168.1.1')
    await user.click(button)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /searching/i })).toBeInTheDocument()
    }, { timeout: 2000 })
  })

  it('handles error state (error message)', async () => {
    const user = userEvent.setup()
    mockLookup.mockRejectedValueOnce(new Error('Network error'))

    render(<Lookup />)

    const textarea = screen.getByPlaceholderText(/enter ip addresses or cidrs/i)
    const button = screen.getByRole('button', { name: /search/i })

    await user.type(textarea, '192.168.1.1')
    await user.click(button)

    await waitFor(() => {
      expect(screen.getByText(/network error/i)).toBeInTheDocument()
    })
  })
})
