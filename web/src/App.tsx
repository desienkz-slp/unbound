import React, { useState, useEffect } from 'react';
import { Header } from './components/Header';
import { Sidebar, NavTab } from './components/Sidebar';
import { Footer } from './components/Footer';
import { ErrorBoundary } from './components/ErrorBoundary';
import { LoginModal } from './components/LoginModal';
import { UserSettingsModal } from './components/UserSettingsModal';
import { OverviewView } from './views/OverviewView';
import { DiagnosticsView } from './views/DiagnosticsView';
import { DNSConfigView } from './views/DNSConfigView';
import { FilteringView } from './views/FilteringView';
import { PerformanceView } from './views/PerformanceView';
import { MonitoringView } from './views/MonitoringView';
import { HistoryView } from './views/HistoryView';
import { AuditView } from './views/AuditView';
import { WizardView } from './views/WizardView';
import { DesiredState, NodeRecord } from './types';
import { api } from './services/api';

export function App() {
  const [currentTab, setCurrentTab] = useState<NavTab>('overview');
  const [nodes, setNodes] = useState<NodeRecord[]>([]);
  const [selectedNodeId, setSelectedNodeId] = useState<string>('');
  const [config, setConfig] = useState<DesiredState | null>(null);
  const [currentUser, setCurrentUser] = useState<string>('admin');
  const [showLogin, setShowLogin] = useState<boolean>(false);
  const [loginNotice, setLoginNotice] = useState<string | null>(null);
  const [showUserSettings, setShowUserSettings] = useState<boolean>(false);
  const [diagnoseDomain, setDiagnoseDomain] = useState<string>('');
  const [unboundHealthy, setUnboundHealthy] = useState<boolean>(true);
  const [inactivityTimeoutMinutes, setInactivityTimeoutMinutes] = useState<number>(() => {
    const saved = localStorage.getItem('netora_inactivity_timeout_minutes');
    return saved !== null ? parseInt(saved, 10) : 15;
  });

  // Initial load
  const loadInitialData = async () => {
    if (!api.getToken()) {
      setShowLogin(true);
      return;
    }

    try {
      const me = await api.getMe();
      if (me && me.username) {
        setCurrentUser(me.username);
      }
    } catch (err: any) {
      if (err.message === 'Unauthorized') {
        setShowLogin(true);
        return;
      }
      console.warn('Failed to verify session token:', err);
    }

    let nodeList: NodeRecord[] = [];
    try {
      nodeList = await api.getNodes();
      setNodes(nodeList);
    } catch (err: any) {
      if (err.message === 'Unauthorized') {
        setShowLogin(true);
        return;
      }
      console.error('Initialization error fetching nodes:', err);
    }

    let activeId = selectedNodeId;
    if (nodeList.length > 0) {
      if (!activeId || !nodeList.some((n) => n.id === activeId)) {
        activeId = nodeList[0].id;
        setSelectedNodeId(activeId);
      }
    } else if (!activeId) {
      // Fallback: discover node_id from system version or use local node fallback
      try {
        const sysVer = await api.getSystemVersion();
        if (sysVer && (sysVer as any).node_id) {
          activeId = (sysVer as any).node_id;
          setSelectedNodeId(activeId);
        }
      } catch {
        // default fallback
        activeId = 'netora-dns1';
        setSelectedNodeId(activeId);
      }
    }

    if (activeId) {
      try {
        const cfg = await api.getDesiredState(activeId);
        setConfig(cfg);
      } catch (err: any) {
        if (err.message === 'Unauthorized') {
          setShowLogin(true);
          return;
        }
        console.error('Initialization error fetching desired state:', err);
      }

      try {
        const health = await api.getNodeHealth(activeId);
        setUnboundHealthy(health.healthy);
      } catch (err: any) {
        console.warn('Failed to load node health:', err);
      }
    }
  };

  useEffect(() => {
    loadInitialData();
  }, [selectedNodeId]);

  const activeNode = nodes.find((n) => n.id === selectedNodeId) || nodes[0] || {
    id: selectedNodeId || 'local-node',
    hostname: window.location.hostname,
    management_ip: window.location.hostname,
    status: 'ONLINE',
    unbound_version: '1.x',
    cpu_cores: 1,
    ram_mb: 1024,
    last_seen_at: new Date().toISOString(),
  };

  const handleQuickDiagnose = (domain: string) => {
    setDiagnoseDomain(domain);
    setCurrentTab('diagnostics');
  };

  const handleLogout = async (reason: string = 'MANUAL', notice?: string | null) => {
    try {
      await api.logout(reason);
    } catch (err) {
      console.warn('Logout error:', err);
    } finally {
      api.setToken(null);
      localStorage.removeItem('netora_last_activity');
      setCurrentUser('');
      setConfig(null);
      setLoginNotice(notice || null);
      setShowLogin(true);
    }
  };

  // User activity tracking & Session Inactivity Auto-Logout
  useEffect(() => {
    if (showLogin || !api.getToken()) return;

    // Initialize activity timestamp
    localStorage.setItem('netora_last_activity', Date.now().toString());

    let throttleTimer: ReturnType<typeof setTimeout> | null = null;
    const recordActivity = () => {
      if (!throttleTimer) {
        localStorage.setItem('netora_last_activity', Date.now().toString());
        throttleTimer = setTimeout(() => {
          throttleTimer = null;
        }, 5000); // Record activity at most once every 5 seconds
      }
    };

    const events = ['mousemove', 'mousedown', 'keydown', 'scroll', 'touchstart'];
    events.forEach((ev) => window.addEventListener(ev, recordActivity, { passive: true }));

    // Heartbeat check for inactivity every 10 seconds
    const interval = setInterval(() => {
      if (inactivityTimeoutMinutes <= 0) return; // 0 = disabled

      const last = parseInt(localStorage.getItem('netora_last_activity') || '0', 10);
      if (!last) return;

      const elapsedMs = Date.now() - last;
      const timeoutMs = inactivityTimeoutMinutes * 60 * 1000;

      if (elapsedMs >= timeoutMs) {
        console.warn(`[AUTH] Session timeout reached (${inactivityTimeoutMinutes}m of inactivity). Auto-logging out...`);
        handleLogout(
          'INACTIVITY_TIMEOUT',
          `Sesi telah berakhir otomatis karena tidak ada aktivitas selama ${inactivityTimeoutMinutes} menit demi keamanan NOC.`
        );
      }
    }, 10000);

    return () => {
      events.forEach((ev) => window.removeEventListener(ev, recordActivity));
      if (throttleTimer) clearTimeout(throttleTimer);
      clearInterval(interval);
    };
  }, [showLogin, inactivityTimeoutMinutes]);

  return (
    <div className="h-screen w-screen bg-[#080C14] text-slate-100 flex flex-col font-sans overflow-hidden">
      <Header
        nodes={nodes.length > 0 ? nodes : [activeNode]}
        selectedNode={selectedNodeId}
        onSelectNode={(id) => setSelectedNodeId(id)}
        onQuickDiagnose={handleQuickDiagnose}
        onLogout={() => handleLogout('MANUAL', null)}
        currentUser={currentUser}
        onOpenUserSettings={() => setShowUserSettings(true)}
      />

      <div className="flex-1 flex overflow-hidden min-h-0">
        <Sidebar
          currentTab={currentTab}
          onSelectTab={setCurrentTab}
          unboundStatus={unboundHealthy}
          onOpenUserSettings={() => setShowUserSettings(true)}
        />

        <main className="flex-1 overflow-y-auto min-h-0 p-6 md:p-8 custom-scrollbar">
          <ErrorBoundary key={currentTab}>
            {currentTab === 'overview' && (
              <OverviewView
                nodeId={selectedNodeId}
                config={config}
                node={activeNode}
                onNavigate={(tab) => setCurrentTab(tab)}
              />
            )}

            {currentTab === 'diagnostics' && (
              <DiagnosticsView initialDomain={diagnoseDomain} />
            )}

            {currentTab === 'dns-config' && (
              <DNSConfigView
                nodeId={selectedNodeId}
                config={config}
                onConfigUpdated={(newCfg) => setConfig(newCfg)}
              />
            )}

            {currentTab === 'filtering' && (
              <FilteringView
                config={config}
                nodeId={selectedNodeId}
                onConfigUpdated={(newCfg) => setConfig(newCfg)}
              />
            )}

            {currentTab === 'performance' && (
              <PerformanceView
                config={config}
                node={activeNode}
                nodeId={selectedNodeId}
                onConfigUpdated={(newCfg) => setConfig(newCfg)}
              />
            )}

            {currentTab === 'monitoring' && (
              <MonitoringView nodeId={selectedNodeId} node={activeNode} />
            )}

            {currentTab === 'history' && (
              <HistoryView
                nodeId={selectedNodeId}
                onRollbackComplete={loadInitialData}
              />
            )}

            {currentTab === 'audit' && <AuditView />}

            {currentTab === 'wizard' && (
              <WizardView
                node={activeNode}
                nodeId={selectedNodeId}
                existingConfig={config}
                onFinish={() => {
                  loadInitialData();
                  setCurrentTab('overview');
                }}
              />
            )}
          </ErrorBoundary>
        </main>
      </div>

      <Footer node={activeNode} unboundHealthy={unboundHealthy} />

      <LoginModal
        isOpen={showLogin}
        noticeMessage={loginNotice}
        onSuccess={(user) => {
          setCurrentUser(user.username);
          setLoginNotice(null);
          setShowLogin(false);
          loadInitialData();
        }}
      />

      <UserSettingsModal
        isOpen={showUserSettings}
        onClose={() => setShowUserSettings(false)}
        currentUser={currentUser}
        onUserUpdated={(newUsername) => {
          setCurrentUser(newUsername);
          loadInitialData();
        }}
        timeoutMinutes={inactivityTimeoutMinutes}
        onTimeoutUpdated={(newTimeout) => {
          setInactivityTimeoutMinutes(newTimeout);
          localStorage.setItem('netora_inactivity_timeout_minutes', newTimeout.toString());
        }}
      />
    </div>
  );
}

export default App;
