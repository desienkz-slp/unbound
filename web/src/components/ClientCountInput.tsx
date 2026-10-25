import React, { useState, useEffect, ChangeEvent } from 'react';
import { MAX_HOMEPASS_COUNT, DEVICES_PER_HOMEPASS } from '../constants/clientTier';

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

  const estimatedDevices = localValue * DEVICES_PER_HOMEPASS;

  return (
    <div className="mb-6">
      <label className="block text-sm font-medium mb-1" htmlFor="homepass-count-input">
        Jumlah Homepass
      </label>
      <div className="flex items-center space-x-3">
        <input
          id="homepass-count-input"
          type="number"
          min={0}
          max={MAX_HOMEPASS_COUNT}
          value={localValue}
          onChange={handleNumberChange}
          className="w-24 px-2 py-1 border rounded bg-gray-800 text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
          aria-label="Jumlah homepass pelanggan"
        />
        <input
          type="range"
          min={0}
          max={MAX_HOMEPASS_COUNT}
          value={localValue}
          onChange={handleSliderChange}
          className="flex-1 client-count-slider"
          aria-label="Slider jumlah homepass"
        />
      </div>
      <p className="mt-1 text-xs text-gray-400">
        ≤500 (Small), 501–2.000 (Medium), &gt;2.000 (Large)
        {localValue > 0 && (
          <span className="ml-2 text-cyan-400">
            ≈ {estimatedDevices.toLocaleString()} device ({DEVICES_PER_HOMEPASS} device/homepass)
          </span>
        )}
      </p>
    </div>
  );
};
