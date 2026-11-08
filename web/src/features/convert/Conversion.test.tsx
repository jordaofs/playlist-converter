import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import { Review } from './Review';
import { Progress } from './Progress';
import { Result } from './Result';
import { PlaylistForm } from './PlaylistForm';
import { job } from '../../test/fixtures';
describe('conversion views', () => {
  it('validates input and connected accounts before submitting', async () => {
    const onSubmit = vi.fn();
    const u = userEvent.setup();
    const { rerender } = render(
      <PlaylistForm
        connected={false}
        busy={false}
        maxItems={100}
        onSubmit={onSubmit}
      />,
    );
    await u.click(screen.getByRole('button', { name: /find my songs/i }));
    expect(screen.getByRole('alert')).toHaveTextContent('valid');
    await u.type(
      screen.getByLabelText('Your playlist link'),
      'https://open.spotify.com/playlist/abc',
    );
    expect(screen.getByText('Spotify')).toBeVisible();
    await u.click(screen.getByRole('button', { name: /find my songs/i }));
    expect(screen.getByRole('alert')).toHaveTextContent('Connect both');
    rerender(
      <PlaylistForm
        connected
        busy={false}
        maxItems={100}
        onSubmit={onSubmit}
      />,
    );
    await u.click(screen.getByRole('button', { name: /find my songs/i }));
    expect(onSubmit).toHaveBeenCalledWith(
      'https://open.spotify.com/playlist/abc',
    );
    rerender(
      <PlaylistForm connected busy maxItems={100} onSubmit={onSubmit} />,
    );
    expect(screen.getByRole('button')).toBeDisabled();
  });
  it('requires resolved selections and disallows unavailable candidates', async () => {
    const decide = vi.fn(),
      publish = vi.fn(),
      cancel = vi.fn();
    const u = userEvent.setup();
    const { rerender } = render(
      <Review
        job={job}
        busy={false}
        onDecision={decide}
        onDecisions={vi.fn()}
        onPublish={publish}
        onCancel={cancel}
      />,
    );
    expect(
      screen.getByRole('button', { name: /create playlist/i }),
    ).toBeDisabled();
    expect(
      screen.getByRole('button', { name: /night drive live/i }),
    ).toBeDisabled();
    await u.click(
      screen.getByRole('button', { name: /night drive\s*artist/i }),
    );
    expect(decide).toHaveBeenCalledWith({
      itemId: '1',
      candidateId: 'y1',
      action: 'choose',
    });
    await u.click(screen.getByRole('button', { name: 'Skip this song' }));
    expect(decide).toHaveBeenCalledWith({ itemId: '1', action: 'skip' });
    const resolved = {
      ...job,
      status: 'ready_to_publish',
      items: [{ ...job.items[0], decision: 'manual', selectedId: 'y1' }],
    };
    rerender(
      <Review
        job={resolved}
        busy={false}
        onDecision={decide}
        onDecisions={vi.fn()}
        onPublish={publish}
        onCancel={cancel}
      />,
    );
    await u.clear(screen.getByLabelText('New playlist name'));
    expect(
      screen.getByRole('button', { name: /create playlist/i }),
    ).toBeDisabled();
    await u.type(screen.getByLabelText('New playlist name'), ' My songs ');
    await u.click(screen.getByRole('button', { name: /create playlist/i }));
    expect(publish).toHaveBeenCalledWith('My songs');
    await u.click(screen.getByRole('button', { name: 'Cancel conversion' }));
    expect(cancel).toHaveBeenCalled();
  });
  it('cannot publish pending or all-skipped items', () => {
    const props = {
      busy: false,
      onDecision: vi.fn(),
      onDecisions: vi.fn(),
      onPublish: vi.fn(),
      onCancel: vi.fn(),
    };
    const { rerender } = render(
      <Review
        {...props}
        job={{ ...job, items: [{ ...job.items[0], decision: 'pending' }] }}
      />,
    );
    expect(screen.getByText(/1 song needs your choice/)).toBeVisible();
    rerender(
      <Review
        {...props}
        job={{
          ...job,
          sourceName: '',
          items: [
            {
              ...job.items[0],
              decision: 'skipped',
              source: { ...job.items[0].source, title: '', artists: [] },
              candidates: [],
            },
          ],
        }}
      />,
    );
    expect(screen.getByText(/Choose at least one/)).toBeVisible();
    expect(screen.getByText('Unavailable song')).toBeVisible();
  });
  it('shows resumable progress and cancellation', async () => {
    const resume = vi.fn(),
      cancel = vi.fn();
    const u = userEvent.setup();
    const { rerender } = render(
      <Progress
        job={{ ...job, status: 'publishing', published: 1 }}
        busy={false}
        onResume={resume}
        onCancel={cancel}
      />,
    );
    expect(screen.getByText('1 songs added')).toBeVisible();
    for (const reason of [
      'quota_exhausted',
      'authorization_required',
      'unknown_remote_outcome',
      'other',
    ]) {
      rerender(
        <Progress
          job={{ ...job, status: 'matching', pauseReason: reason }}
          busy={false}
          onResume={resume}
          onCancel={cancel}
        />,
      );
      expect(screen.getByRole('status')).toBeVisible();
    }
    await u.click(screen.getByRole('button', { name: /Resume/ }));
    expect(resume).toHaveBeenCalled();
    await u.click(screen.getByRole('button', { name: /Cancel/ }));
    expect(cancel).toHaveBeenCalled();
    rerender(
      <Progress
        job={{ ...job, status: 'unexpected' }}
        busy
        onResume={resume}
        onCancel={cancel}
      />,
    );
    expect(screen.getByText('Preparing your playlist')).toBeVisible();
  });
  it('reports completion, partial progress, failure and cancellation honestly', async () => {
    const fresh = vi.fn(),
      resume = vi.fn();
    const u = userEvent.setup();
    const props = { busy: false, onNew: fresh, onResume: resume };
    const items = ['automatic', 'manual', 'skipped', 'unavailable'].map(
      (decision, i) => ({
        ...job.items[0],
        id: String(i),
        decision,
        selectedId: i < 2 ? 'y1' : undefined,
      }),
    );
    const { rerender } = render(
      <Result
        {...props}
        job={{
          ...job,
          status: 'completed',
          published: 2,
          items,
          destinationUrl: 'https://www.youtube.com/playlist?list=x',
        }}
      />,
    );
    expect(screen.getByRole('link', { name: /Open playlist/ })).toHaveAttribute(
      'href',
      'https://www.youtube.com/playlist?list=x',
    );
    expect(screen.getByRole('status')).toHaveTextContent('2 songs');
    await u.click(screen.getByRole('button', { name: /Start another/ }));
    expect(fresh).toHaveBeenCalled();
    rerender(
      <Result
        {...props}
        job={{ ...job, status: 'partial', pauseReason: 'quota_exhausted' }}
      />,
    );
    await u.click(screen.getByRole('button', { name: /Resume/ }));
    expect(resume).toHaveBeenCalled();
    rerender(
      <Result
        {...props}
        job={{ ...job, status: 'failed', error: 'conversion_failed' }}
      />,
    );
    expect(screen.getByRole('status')).toHaveTextContent(
      'Something went wrong on our side. Your original playlist is unchanged.',
    );
    rerender(
      <Result
        {...props}
        job={{ ...job, status: 'failed', error: 'access_denied' }}
      />,
    );
    expect(screen.getByRole('status')).toHaveTextContent(
      'The music service refused access to this playlist. Only playlists you created with the connected account can be converted.',
    );
    expect(screen.queryByRole('button', { name: /Resume/ })).toBeNull();
    rerender(
      <Result
        {...props}
        job={{ ...job, status: 'failed', error: 'constructor' }}
      />,
    );
    expect(screen.getByRole('status')).toHaveTextContent(
      'Your original playlist is unchanged.',
    );
    rerender(<Result {...props} job={{ ...job, status: 'cancelled' }} />);
    expect(screen.getByText('No more changes will be made.')).toBeVisible();
  });
});

describe('long playlists', () => {
  const song = (id: string, unavailable?: string) => ({
    song: {
      id,
      title: `Song ${id}`,
      artists: ['Artist'],
      durationMs: 180000,
      url: `https://www.youtube.com/watch?v=${id}`,
      ...(unavailable ? { unavailable } : {}),
    },
    score: 0.8,
    reasons: ['Similar title'],
  });
  const base = job.items[0];
  const items = [
    base,
    {
      ...base,
      id: '2',
      position: 1,
      candidates: [song('z1', 'region'), song('z2')],
    },
    {
      ...base,
      id: '3',
      position: 2,
      decision: 'automatic',
      selectedId: 'y1',
    },
    { ...base, id: '4', position: 3, candidates: [song('w1', 'deleted')] },
  ];
  const props = {
    busy: false,
    onDecision: vi.fn(),
    onPublish: vi.fn(),
    onCancel: vi.fn(),
  };

  it('selects the best available match for every song that needs a choice', async () => {
    const onDecisions = vi.fn();
    const u = userEvent.setup();
    render(
      <Review {...props} job={{ ...job, items }} onDecisions={onDecisions} />,
    );
    await u.click(
      screen.getByRole('button', { name: 'Use best match for all (2)' }),
    );
    expect(onDecisions).toHaveBeenCalledTimes(1);
    expect(onDecisions).toHaveBeenCalledWith([
      { itemId: '1', candidateId: 'y1', action: 'choose' },
      { itemId: '2', candidateId: 'z2', action: 'choose' },
    ]);
    expect(
      screen.getByText('You can still change any song below.'),
    ).toBeVisible();
  });

  it('hides the bulk action when nothing can be selected', () => {
    render(
      <Review
        {...props}
        job={{ ...job, items: [items[2], items[3]] }}
        onDecisions={vi.fn()}
      />,
    );
    expect(screen.queryByRole('button', { name: /Use best match/ })).toBeNull();
  });

  it('explains a paused publish instead of showing its code', () => {
    render(
      <Result
        job={{ ...job, status: 'partial', pauseReason: 'not_found' }}
        busy={false}
        onNew={vi.fn()}
        onResume={vi.fn()}
      />,
    );
    expect(screen.getByRole('status')).toHaveTextContent(
      'YouTube hasn’t made the new playlist available yet. Resume in a minute to keep adding songs.',
    );
    expect(screen.queryByText('not_found')).toBeNull();
  });
});
