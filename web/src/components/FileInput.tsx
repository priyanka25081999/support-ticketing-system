import { useRef } from 'react';
import { MAX_FILE_BYTES, MAX_FILES, formatBytes } from '../utils';

interface Props {
  files: File[];
  onChange: (files: File[]) => void;
  onError: (message: string) => void;
}

/** Multi-file picker with client-side size/count checks and removable chips. */
export function FileInput({ files, onChange, onError }: Props) {
  const inputRef = useRef<HTMLInputElement>(null);

  const addFiles = (list: FileList | null) => {
    const picked = Array.from(list ?? []);
    const tooBig = picked.find((f) => f.size > MAX_FILE_BYTES);
    if (tooBig) onError(`${tooBig.name} is larger than ${formatBytes(MAX_FILE_BYTES)}`);
    const next = [...files, ...picked.filter((f) => f.size <= MAX_FILE_BYTES && f.size > 0)].slice(0, MAX_FILES);
    onChange(next);
    if (inputRef.current) inputRef.current.value = ''; // allow re-selecting the same file
  };

  return (
    <div className="file-input">
      <button type="button" className="btn btn-small" onClick={() => inputRef.current?.click()}>
        📎 Add files
      </button>
      <span className="muted">Up to {MAX_FILES} files, {formatBytes(MAX_FILE_BYTES)} each</span>
      <input ref={inputRef} type="file" multiple hidden aria-label="Attachments" onChange={(e) => addFiles(e.target.files)} />
      {files.length > 0 && (
        <ul className="file-chips">
          {files.map((f, i) => (
            <li key={`${f.name}-${i}`}>
              {f.name} <span className="muted">({formatBytes(f.size)})</span>
              <button type="button" aria-label={`Remove ${f.name}`} onClick={() => onChange(files.filter((_, j) => j !== i))}>
                ×
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
