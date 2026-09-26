import React, { Component, ErrorInfo, ReactNode } from 'react';
import { AlertTriangle, RefreshCw, RotateCcw } from 'lucide-react';

interface Props {
  children: ReactNode;
  fallbackTitle?: string;
}

interface State {
  hasError: boolean;
  error: Error | null;
  errorInfo: ErrorInfo | null;
}

export class ErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false,
    error: null,
    errorInfo: null,
  };

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error, errorInfo: null };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('ErrorBoundary caught an error:', error, errorInfo);
    this.setState({ errorInfo });
  }

  private handleReset = () => {
    this.setState({ hasError: false, error: null, errorInfo: null });
  };

  private handleReload = () => {
    window.location.reload();
  };

  public render() {
    if (this.state.hasError) {
      return (
        <div className="p-6 max-w-3xl mx-auto my-8">
          <div className="bg-gray-900 border border-rose-900/40 rounded-2xl p-6 shadow-2xl relative overflow-hidden backdrop-blur-md">
            <div className="absolute top-0 right-0 w-64 h-64 bg-rose-500/10 rounded-full blur-3xl pointer-events-none" />
            
            <div className="flex items-start gap-4 relative z-10">
              <div className="p-3 bg-rose-950/60 border border-rose-800 text-rose-400 rounded-xl shrink-0">
                <AlertTriangle className="w-6 h-6 animate-pulse" />
              </div>

              <div className="flex-1 space-y-3">
                <div>
                  <h3 className="text-base font-bold text-white tracking-wide">
                    {this.props.fallbackTitle || 'Terjadi Kendala Rendering Komponen'}
                  </h3>
                  <p className="text-xs text-gray-400 mt-1">
                    Komponen ini mengalami runtime error dan dilindungi oleh Error Boundary agar tidak merusak tampilan sistem keseluruhan.
                  </p>
                </div>

                {this.state.error && (
                  <div className="p-3.5 bg-gray-950/80 border border-rose-900/30 rounded-xl font-mono text-xs text-rose-300 overflow-x-auto">
                    <span className="font-semibold text-rose-400">Error: </span>
                    {this.state.error.toString()}
                  </div>
                )}

                <div className="flex items-center gap-3 pt-2">
                  <button
                    onClick={this.handleReset}
                    className="px-4 py-2 bg-gradient-to-r from-cyan-500 to-blue-600 hover:from-cyan-400 hover:to-blue-500 text-black font-bold text-xs rounded-xl transition-all shadow-md flex items-center gap-2"
                  >
                    <RotateCcw className="w-3.5 h-3.5" />
                    Coba Muat Ulang View
                  </button>

                  <button
                    onClick={this.handleReload}
                    className="px-4 py-2 bg-gray-800 hover:bg-gray-750 text-gray-200 border border-gray-700 hover:border-gray-600 font-medium text-xs rounded-xl transition-all flex items-center gap-2"
                  >
                    <RefreshCw className="w-3.5 h-3.5 text-gray-400" />
                    Refresh Halaman
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
