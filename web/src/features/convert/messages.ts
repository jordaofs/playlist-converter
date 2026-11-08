// Readable text for conversion codes. Screens never show raw codes: unknown
// codes return '' so callers can fall back to a neutral sentence.

// Reasons a conversion is on hold (`job.pauseReason`).
const pauseMessages: Record<string, string> = {
  quota_exhausted:
    'The music service needs a little time. Your progress is saved.',
  authorization_required: 'Reconnect the same music account to keep going.',
  unknown_remote_outcome:
    'We’re checking whether the last change reached the music service. Your progress is saved.',
  not_found:
    'YouTube hasn’t made the new playlist available yet. Resume in a minute to keep adding songs.',
  destination_changed:
    'The new playlist was changed outside this site. Check it, then resume.',
  provider_unavailable:
    'The music service is having trouble. Your progress is saved; resume in a few minutes.',
};

// Reasons a conversion stopped for good (`job.error`).
const failureMessages: Record<string, string> = {
  access_denied:
    'The music service refused access to this playlist. Only playlists you created with the connected account can be converted.',
  not_found:
    'We couldn’t find this playlist. Check the link and make sure it’s one you created.',
  playlist_not_owned:
    'This playlist belongs to someone else. Only playlists you created can be converted.',
  empty_playlist: 'This playlist has no songs to convert.',
  playlist_too_large:
    'This playlist has more songs than the current limit of 100.',
  source_changed:
    'The playlist changed while we were reading it. Start again to use the latest version.',
  api_not_enabled:
    'The music service isn’t available for this site right now. Please try again later.',
  conversion_failed:
    'Something went wrong on our side. Your original playlist is unchanged.',
};

function lookup(messages: Record<string, string>, code?: string) {
  return code && Object.hasOwn(messages, code) ? messages[code] : '';
}

export function pauseMessage(code?: string) {
  return lookup(pauseMessages, code);
}

export function failureMessage(code?: string) {
  return lookup(failureMessages, code);
}
