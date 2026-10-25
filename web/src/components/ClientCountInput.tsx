import React, { useState, useEffect, ChangeEvent } from 'react';

interface ClientCountInputProps {
  value: number;
  onChange: (v: number) => void;
}

export const ClientCountInput: React.FC<ClientCountInputProps> = ({ value, onChange }) => {
  const [localValue, setLocalValue] = useState<number>(value);

  // Debounce updates to parent
  useEffect(() => {
    const handler = setTimeout(() => onChange(localValue), 300);
    return () => clearTimeout(handler);
  }, [localValue, onChange]);

  const handleNumberChange = (e: ChangeEvent<HTMLInputElement>) => {
    const v = parseInt(e.target.value, 10);
    if (!isNaN(v) && v >= 0) setLocalValue(v);
  };

  const handleSliderChange = (e: ChangeEvent<HTMLInputElement>) => {
    setLocalValue(parseInt(e.target.value, 10));
  };

  return (
    <div className="mb-6">
      <label className="block text-sm font-medium mb-1" htmlFor="client-count-input">
        Client Subscriber Count
      </label>
      <div className="flex items-center space-x-3">
        <input
          id="client-count-input"
          type="number"
          min={0}
          max={5000}
          value={localValue}
          onChange={handleNumberChange}
          className="w-24 px-2 py-1 border rounded bg-gray-800 text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
          aria-label="Number of client subscribers"
        />
        <input
          type="range"
          min={0}
          max={5000}
          value={localValue}
          onChange={handleSliderChange}
          className="flex-1 client-count-slider"
          aria-label="Client subscriber count slider"
        />
      </div>
      <p className="mt-1 text-xs text-gray-400">
        Recommended: 0‑500 (Small), 501‑2000 (Medium), &gt;2000 (Large)
      </p>
    </div>
  );
};
