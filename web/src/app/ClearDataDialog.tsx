import { useEffect, useRef } from 'react';
export function ClearDataDialog({
  onClose,
  onConfirm,
}: {
  onClose: () => void;
  onConfirm: () => void;
}) {
  const dialog = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    const root = dialog.current;
    const buttons = root?.querySelectorAll<HTMLButtonElement>('button');
    buttons?.[0]?.focus();
    const key = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        onClose();
      }
      if (event.key === 'Tab' && buttons?.length) {
        const first = buttons[0],
          last = buttons[buttons.length - 1];
        if (event.shiftKey && document.activeElement === first) {
          event.preventDefault();
          last.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault();
          first.focus();
        }
      }
    };
    document.addEventListener('keydown', key);
    return () => {
      document.removeEventListener('keydown', key);
      previous?.focus();
    };
  }, [onClose]);
  return (
    <div className="dialog-backdrop" ref={dialog}>
      <section
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="clear-title"
        aria-describedby="clear-description"
        className="dialog"
      >
        <h2 id="clear-title">Clear your saved data?</h2>
        <p id="clear-description">
          This disconnects your accounts and removes saved progress from this
          app. Playlists already created on Spotify or YouTube stay there.
        </p>
        <div className="button-row">
          <button className="secondary-button" onClick={onClose}>
            Keep my data
          </button>
          <button className="primary-button compact" onClick={onConfirm}>
            Clear my data
          </button>
        </div>
      </section>
    </div>
  );
}
