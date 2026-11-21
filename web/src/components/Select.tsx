import { KeyboardEvent, ReactNode, useEffect, useId, useRef, useState } from 'react';

export interface SelectOption {
  value: string;
  label: string;
}

interface SelectProps {
  value: string;
  options: SelectOption[];
  onChange: (value: string) => void;
  /** Accessible name, e.g. "Status". */
  label: string;
  placeholder?: string;
  disabled?: boolean;
  invalid?: boolean;
  icon?: ReactNode;
  /** Align the menu to the right edge of the trigger (for controls near the right of the screen). */
  alignRight?: boolean;
  className?: string;
}

const Chevron = () => (
  <svg className="select-chevron" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" aria-hidden="true">
    <polyline points="6 9 12 15 18 9" />
  </svg>
);

const Check = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" aria-hidden="true">
    <polyline points="20 6 9 17 4 12" />
  </svg>
);

/**
 * Lightweight accessible dropdown (WAI-ARIA listbox pattern):
 * click or Enter/Space/Arrow keys to open, arrows/Home/End to move,
 * Enter to pick, Escape/Tab/outside click to close, type a letter to jump.
 */
export function Select({ value, options, onChange, label, placeholder = 'Select…', disabled, invalid, icon, alignRight, className = '' }: SelectProps) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const rootRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const id = useId();
  const selectedIndex = options.findIndex((o) => o.value === value);
  const selected = options[selectedIndex];

  const openMenu = () => {
    if (disabled || options.length === 0) return;
    setActive(Math.max(0, selectedIndex));
    setOpen(true);
  };
  const close = (focusButton = true) => {
    setOpen(false);
    if (focusButton) buttonRef.current?.focus();
  };
  const choose = (index: number) => {
    const option = options[index];
    if (option && option.value !== value) onChange(option.value);
    close();
  };

  useEffect(() => {
    if (open) listRef.current?.focus();
  }, [open]);

  useEffect(() => {
    if (open) document.getElementById(`${id}-${active}`)?.scrollIntoView?.({ block: 'nearest' });
  }, [open, active, id]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [open]);

  const onButtonKeyDown = (e: KeyboardEvent) => {
    if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(e.key)) {
      e.preventDefault();
      openMenu();
    }
  };

  const onListKeyDown = (e: KeyboardEvent) => {
    const last = options.length - 1;
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setActive((i) => Math.min(last, i + 1));
        break;
      case 'ArrowUp':
        e.preventDefault();
        setActive((i) => Math.max(0, i - 1));
        break;
      case 'Home':
        e.preventDefault();
        setActive(0);
        break;
      case 'End':
        e.preventDefault();
        setActive(last);
        break;
      case 'Enter':
      case ' ':
        e.preventDefault();
        choose(active);
        break;
      case 'Escape':
        e.preventDefault();
        close();
        break;
      case 'Tab':
        close(false);
        break;
      default:
        // Type-ahead: jump to the next option starting with the typed letter.
        if (e.key.length === 1) {
          const key = e.key.toLowerCase();
          const order = [...options.slice(active + 1), ...options.slice(0, active + 1)];
          const match = order.find((o) => o.label.toLowerCase().startsWith(key));
          if (match) setActive(options.indexOf(match));
        }
    }
  };

  return (
    <div className={`select ${className}`} ref={rootRef}>
      <button
        type="button"
        ref={buttonRef}
        className={`select-trigger ${invalid ? 'invalid' : ''} ${open ? 'open' : ''}`}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={`${label}: ${selected?.label ?? placeholder}`}
        disabled={disabled}
        onClick={() => (open ? close() : openMenu())}
        onKeyDown={onButtonKeyDown}
      >
        {icon}
        <span className={`select-value ${selected ? '' : 'placeholder'}`}>{selected?.label ?? placeholder}</span>
        <Chevron />
      </button>
      {open && (
        <ul
          ref={listRef}
          role="listbox"
          tabIndex={-1}
          aria-label={label}
          aria-activedescendant={`${id}-${active}`}
          className={`select-menu ${alignRight ? 'align-right' : ''}`}
          onKeyDown={onListKeyDown}
        >
          {options.map((o, i) => (
            <li
              key={o.value}
              id={`${id}-${i}`}
              role="option"
              aria-selected={o.value === value}
              className={`select-option ${i === active ? 'active' : ''} ${o.value === value ? 'selected' : ''}`}
              onMouseEnter={() => setActive(i)}
              onMouseDown={(e) => e.preventDefault()} // keep focus in the list
              onClick={() => choose(i)}
            >
              <span>{o.label}</span>
              {o.value === value && <Check />}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
