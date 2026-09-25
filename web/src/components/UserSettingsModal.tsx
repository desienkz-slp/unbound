import React, { useState, useEffect } from 'react';
import {
  AlertCircle,
  CheckCircle2,
  Eye,
  EyeOff,
  KeyRound,
  Lock,
  ShieldCheck,
  User,
  UserCheck,
  X,
} from 'lucide-react';
import { api } from '../services/api';

interface UserSettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
  currentUser: string;
  onUserUpdated: (newUsername: string) => void;
}

export const UserSettingsModal: React.FC<UserSettingsModalProps> = ({
  isOpen,
  onClose,
  currentUser,
  onUserUpdated,
}) => {
  const [username, setUsername] = useState(currentUser || 'admin');
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');

  const [showCurrentPass, setShowCurrentPass] = useState(false);
  const [showNewPass, setShowNewPass] = useState(false);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen) {
      setUsername(currentUser || 'admin');
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      setError(null);
      setSuccessMsg(null);
    }
  }, [isOpen, currentUser]);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSuccessMsg(null);

    // Frontend validations
    const trimmedUsername = username.trim();
    if (!trimmedUsername) {
      setError('Username tidak boleh kosong.');
      return;
    }
    if (trimmedUsername.length < 3) {
      setError('Username minimal harus 3 karakter.');
      return;
    }

    if (!currentPassword) {
      setError('Password saat ini wajib diisi untuk verifikasi keamanan.');
      return;
    }

    if (newPassword) {
      if (newPassword.length < 6) {
        setError('Password baru minimal harus 6 karakter.');
        return;
      }
      if (newPassword !== confirmPassword) {
        setError('Konfirmasi password baru tidak cocok.');
        return;
      }
    }

    setLoading(true);
    try {
      const res = await api.updateProfile({
        username: trimmedUsername,
        current_password: currentPassword,
        new_password: newPassword ? newPassword : undefined,
      });

      setSuccessMsg('Pengaturan akun berhasil disimpan.');
      onUserUpdated(res.user.username);
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');

      setTimeout(() => {
        onClose();
      }, 1500);
    } catch (err: any) {
      setError(err.message || 'Gagal memperbarui pengaturan akun.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/80 backdrop-blur-md flex items-center justify-center p-4 z-50 animate-in fade-in duration-200">
      <div className="bg-[#0B0F19] border border-gray-800 rounded-2xl max-w-md w-full p-6 sm:p-7 shadow-2xl relative overflow-hidden">
        {/* Ambient background glow */}
        <div className="absolute top-0 right-0 -mr-16 -mt-16 w-48 h-48 bg-cyan-500/10 rounded-full blur-3xl pointer-events-none" />

        {/* Modal Header */}
        <div className="flex items-start justify-between pb-4 border-b border-gray-800">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400">
              <UserCheck className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-base font-bold text-white tracking-wide">Pengaturan Akun & Keamanan</h2>
              <p className="text-[11px] text-gray-400">Perbarui username & password operator Netora</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 text-gray-400 hover:text-white hover:bg-gray-800 rounded-lg transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Account Info Card */}
        <div className="my-4 p-3 bg-gray-900/80 border border-gray-800 rounded-xl flex items-center justify-between text-xs font-mono">
          <div className="flex items-center gap-2">
            <span className="text-gray-400">Akun Aktif:</span>
            <span className="text-cyan-400 font-bold">{currentUser}</span>
          </div>
          <span className="px-2 py-0.5 rounded-full bg-emerald-950/80 text-emerald-400 border border-emerald-800 text-[10px] font-semibold flex items-center gap-1">
            <ShieldCheck className="w-3 h-3" /> SUPER_ADMIN
          </span>
        </div>

        {/* Notifications */}
        {error && (
          <div className="mb-4 p-3 bg-red-950/70 border border-red-800 rounded-xl text-xs text-red-300 flex items-center gap-2.5">
            <AlertCircle className="w-4 h-4 shrink-0 text-red-400" />
            <span>{error}</span>
          </div>
        )}

        {successMsg && (
          <div className="mb-4 p-3 bg-emerald-950/70 border border-emerald-800 rounded-xl text-xs text-emerald-300 flex items-center gap-2.5">
            <CheckCircle2 className="w-4 h-4 shrink-0 text-emerald-400" />
            <span>{successMsg}</span>
          </div>
        )}

        {/* Edit Form */}
        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Username */}
          <div>
            <label className="block text-xs font-semibold text-gray-300 mb-1">
              Username Baru
            </label>
            <div className="relative">
              <User className="w-4 h-4 text-gray-500 absolute left-3 top-2.5" />
              <input
                type="text"
                required
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="Masukkan username baru..."
                className="w-full bg-gray-950 border border-gray-800 rounded-lg pl-9 pr-3 py-2 text-xs text-white placeholder-gray-600 focus:outline-none focus:border-cyan-500 font-mono transition-colors"
              />
            </div>
            <span className="text-[10px] text-gray-500 mt-1 block">
              Digunakan saat login ke Netora DNS NOC (minimal 3 karakter).
            </span>
          </div>

          <div className="pt-2 border-t border-gray-800/80">
            <span className="text-[11px] font-semibold uppercase tracking-wider text-cyan-400 block mb-3">
              Ganti Password (Opsional)
            </span>

            {/* New Password */}
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-semibold text-gray-300 mb-1">
                  Password Baru
                </label>
                <div className="relative">
                  <KeyRound className="w-4 h-4 text-gray-500 absolute left-3 top-2.5" />
                  <input
                    type={showNewPass ? 'text' : 'password'}
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    placeholder="Kosongkan jika tidak ingin ganti password"
                    className="w-full bg-gray-950 border border-gray-800 rounded-lg pl-9 pr-9 py-2 text-xs text-white placeholder-gray-600 focus:outline-none focus:border-cyan-500 font-mono transition-colors"
                  />
                  <button
                    type="button"
                    onClick={() => setShowNewPass(!showNewPass)}
                    className="absolute right-3 top-2.5 text-gray-500 hover:text-gray-300"
                  >
                    {showNewPass ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                </div>
              </div>

              {/* Confirm New Password */}
              {newPassword && (
                <div>
                  <label className="block text-xs font-semibold text-gray-300 mb-1">
                    Konfirmasi Password Baru
                  </label>
                  <div className="relative">
                    <Lock className="w-4 h-4 text-gray-500 absolute left-3 top-2.5" />
                    <input
                      type={showNewPass ? 'text' : 'password'}
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      placeholder="Ketik ulang password baru..."
                      className="w-full bg-gray-950 border border-gray-800 rounded-lg pl-9 pr-3 py-2 text-xs text-white placeholder-gray-600 focus:outline-none focus:border-cyan-500 font-mono transition-colors"
                    />
                  </div>
                  {confirmPassword && newPassword !== confirmPassword && (
                    <span className="text-[10px] text-rose-400 mt-1 block">
                      Password tidak cocok!
                    </span>
                  )}
                </div>
              )}
            </div>
          </div>

          {/* Current Password (Required for confirmation) */}
          <div className="pt-2 border-t border-gray-800/80">
            <label className="block text-xs font-semibold text-gray-300 mb-1">
              Password Saat Ini <span className="text-rose-400">*</span>
            </label>
            <div className="relative">
              <Lock className="w-4 h-4 text-gray-500 absolute left-3 top-2.5" />
              <input
                type={showCurrentPass ? 'text' : 'password'}
                required
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                placeholder="Masukkan password saat ini untuk konfirmasi..."
                className="w-full bg-gray-950 border border-gray-800 rounded-lg pl-9 pr-9 py-2 text-xs text-white placeholder-gray-600 focus:outline-none focus:border-cyan-500 font-mono transition-colors"
              />
              <button
                type="button"
                onClick={() => setShowCurrentPass(!showCurrentPass)}
                className="absolute right-3 top-2.5 text-gray-500 hover:text-gray-300"
              >
                {showCurrentPass ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
            <span className="text-[10px] text-gray-500 mt-1 block">
              Diperlukan untuk memvalidasi otorisasi perubahan kredensial.
            </span>
          </div>

          {/* Action Buttons */}
          <div className="pt-3 flex items-center justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              disabled={loading}
              className="px-4 py-2 bg-gray-900 hover:bg-gray-800 text-gray-300 text-xs font-medium rounded-lg border border-gray-700 transition-all disabled:opacity-50"
            >
              Batal
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-5 py-2 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-black font-bold text-xs rounded-lg transition-all shadow-lg shadow-cyan-500/20 disabled:opacity-50"
            >
              {loading ? 'Menyimpan Perubahan...' : 'Simpan Pengaturan'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
