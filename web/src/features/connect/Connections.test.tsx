import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { it, expect, vi } from 'vitest';
import { Connections } from './Connections';
import { session } from '../../test/fixtures';
it('connects with provider links and shows identities and disconnect controls', async () => {
  const disconnect = vi.fn();
  const { rerender } = render(
    <Connections connections={[]} busy={false} onDisconnect={disconnect} />,
  );
  expect(screen.getByRole('link', { name: /Connect Spotify/ })).toHaveAttribute(
    'href',
    '/api/auth/spotify/start',
  );
  expect(screen.getByRole('link', { name: /Connect YouTube/ })).toHaveAttribute(
    'href',
    '/api/auth/youtube/start',
  );
  rerender(
    <Connections
      connections={session.connections}
      busy={false}
      onDisconnect={disconnect}
    />,
  );
  expect(screen.getByText('Spotify listener')).toBeVisible();
  await userEvent.click(
    screen.getByRole('button', { name: 'Disconnect Spotify' }),
  );
  expect(disconnect).toHaveBeenCalledWith('spotify');
  rerender(
    <Connections
      connections={session.connections}
      busy
      onDisconnect={disconnect}
    />,
  );
  expect(
    screen.getByRole('button', { name: 'Disconnect YouTube' }),
  ).toBeDisabled();
});
