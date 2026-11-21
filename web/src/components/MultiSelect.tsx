import { KeyboardEvent, useEffect, useId, useRef, useState } from 'react';
import type { SelectOption } from './Select';

interface MultiSelectProps {
  values: string[];
  options: SelectOption[];
  onChange: (values: string[]) => void;
  label: string;
  /** Shown when nothing is selected (= no filter), e.g. "All statuses". */
  allLabel: string;
}

/** Checkbox dropdown for multi-value filters. Stays open while toggling options. */
export function MultiSelect({ values, options, onChange, label, allLabel }: MultiSelectProps) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const rootRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const id = useId();

  // Local copy of the selection so rapid toggles build on the latest choice
  // even before the parent (URL state) re-renders with the new value.
  const [selected, setSelected] = useState(values);
  const latest = useRef(values);
  const valuesKey = values.join(',');
  useEffect(() => {
    latest.current = values;
    setSelected(values);
  }, [valuesKey]);

  const selectedLabels = options.filter((o) => selected.includes(o.value)).map((o) => o.label);
  const summary =
    selectedLabels.length === 0 ? allLabel : selectedLabels.length <= 2 ? selectedLabels.join(', ') : `${selectedLabels.length} selected`;

  const update = (next: string[]) => {
    latest.current = next;
    setSelected(next);
    onChange(next);
  };
  const toggle = (value: string) => {
    const current = latest.current;
    update(current.includes(value) ? current.filter((v) => v !== value) : [...current, value]);
  };

  useEffect(() => {
    if (open) listRef.current?.focus();
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [open]);

  const close = () => {
    setOpen(false);
    buttonRef.current?.focus();
  };

  const onListKeyDown = (e: KeyboardEvent) => {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setActive((i) => Math.min(options.length - 1, i + 1));
        break;
      case 'ArrowUp':
        e.preventDefault();
        setActive((i) => Math.max(0, i - 1));
        break;
      case ' ':
      case 'Enter':
        e.preventDefault();
        if (options[active]) toggle(options[active].value);
        break;
      case 'Escape':
        e.preventDefault();
        close();
        break;
      case 'Tab':
        setOpen(false);
    }
  };

  return (
    <div className="select" ref={rootRef}>
      <button
        type="button"
        ref={buttonRef}
        className={`select-trigger ${open ? 'open' : ''} ${selected.length ? 'has-value' : ''}`}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={`${label}: ${summary}`}
        onClick={() => (open ? close() : (setActive(0), setOpen(true)))}
        onKeyDown={(e) => {
          if (['ArrowDown', 'Enter', ' '].includes(e.key)) {
            e.preventDefault();
            setActive(0);
            setOpen(true);
          }
        }}
      >
        <span className="select-value">{summary}</span>
        {selected.length > 0 && <span className="select-count">{selected.length}</span>}
        <svg className="select-chevron" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" aria-hidden="true">
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </button>
      {open && (
        <div className="select-menu">
          <ul
            ref={listRef}
            role="listbox"
            aria-multiselectable="true"
            aria-label={label}
            tabIndex={-1}
            aria-activedescendant={`${id}-${active}`}
            onKeyDown={onListKeyDown}
            className="multi-list"
          >
            {options.map((o, i) => {
              const checked = selected.includes(o.value);
              return (
                <li
                  key={o.value}
                  id={`${id}-${i}`}
                  role="option"
                  aria-selected={checked}
                  className={`select-option multi ${i === active ? 'active' : ''}`}
                  onMouseEnter={() => setActive(i)}
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => toggle(o.value)}
                >
                  <span className={`fake-check ${checked ? 'on' : ''}`} aria-hidden="true" />
                  <span>{o.label}</span>
                </li>
              );
            })}
          </ul>
          {selected.length > 0 && (
            <button type="button" className="multi-clear" onMouseDown={(e) => e.preventDefault()} onClick={() => update([])}>
              Clear selection
            </button>
          )}
        </div>
      )}
    </div>
  );
}
