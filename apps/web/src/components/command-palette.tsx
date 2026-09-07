"use client";

import { Icons } from "./icons";

const actions = [
  ["Go to Payments", "G P"],
  ["Open routing decisions", "G R"],
  ["View degraded providers", "G D"],
  ["Find payment by ID", "⌘ P"],
  ["Connect provider", "C P"],
  ["Replay webhook", "R W"],
];

export function CommandPalette({ open, onClose }: { open: boolean; onClose: () => void }) {
  if (!open) return null;
  return (
    <div className="command-backdrop" role="presentation" onMouseDown={onClose}>
      <div className="command-panel" role="dialog" aria-modal="true" aria-label="Command palette" onMouseDown={(event) => event.stopPropagation()}>
        <div className="command-input-wrap">
          <Icons.search className="icon" />
          <input autoFocus aria-label="Search commands" placeholder="Search Conver Pay…" />
          <kbd>Esc</kbd>
        </div>
        <div className="command-section-label">Suggestions</div>
        <div className="command-actions">
          {actions.map(([label, shortcut], index) => (
            <button className={index === 0 ? "command-row active" : "command-row"} key={label} onClick={onClose}>
              <span><Icons.command className="icon" />{label}</span>
              <kbd>{shortcut}</kbd>
            </button>
          ))}
        </div>
        <div className="command-footer"><span>Navigate <kbd>↑</kbd><kbd>↓</kbd></span><span>Select <kbd>↵</kbd></span></div>
      </div>
    </div>
  );
}
