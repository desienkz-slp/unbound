import React, { useState, useEffect } from 'react';
import { Header } from './components/Header';
import { Sidebar, NavTab } from './components/Sidebar';
import { LoginModal } from './components/LoginModal';
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
  const [selectedNodeId, setSelectedNodeId] = useState<string>('dns-dev');
  const [config, setConfig] = useState<DesiredState | null>(null);
  const [currentUser, setCurrentUser] = useState<string>('admin');
  const [showLogin, setShowLogin] = useState<boolean>(false);
  const [diagnoseDomain, setDiagnoseDomain] = useState<string>('');
  const [unboundHealthy, setUnboundHealthy] = useState<boolean>(true);

  // Initial load
  const loadInitialData = async () => {
    try {
      const nodeList = await api.getNodes();
      setNodes(nodeList);
      if (nodeList.length > 0 && !selectedNodeId) {
        setSelectedNodeId(nodeList[0].id);
      }

      const activeId = selectedNodeId || (nodeList[0]?.id ?? 'dns-dev');
      const cfg = await api.getDesiredState(activeId);
      setConfig(cfg);

      const health = await api.getNodeHealth(activeId);
      setUnboundHealthy(health.healthy);
    } catch (err: any) {
      if (err.message === 'Unauthorized') {
        setShowLogin(true);
      } else {
        console.error('Initialization error:', err);
      }
    }
  };

  useEffect(() => {
    loadInitialData();
  }, [selectedNodeId]);

  const activeNode = nodes.find((n) => n.id === selectedNodeId) || {
    id: 'dns-dev',
    hostname: 'dns-dev',
    management_ip: '172.18.20.145',
    status: 'ONLINE',
    unbound_version: '1.13.1',
    cpu_cores: 2,
    ram_mb: 2048,
    last_seen_at: new Date().toISOString(),
  };

  const handleQuickDiagnose = (domain: string) => {
    setDiagnoseDomain(domain);
    setCurrentTab('diagnostics');
  };

  const handleLogout = async () => {
    await api.logout();
    setShowLogin(true);
  };

  return (
    <div className="min-h-screen bg-[#080C14] text-slate-100 flex flex-col font-sans">
      <Header
        nodes={nodes.length > 0 ? nodes : [activeNode]}
        selectedNode={selectedNodeId}
        onSelectNode={(id) => setSelectedNodeId(id)}
        onQuickDiagnose={handleQuickDiagnose}
        onLogout={handleLogout}
        currentUser={currentUser}
      />

      <div className="flex-1 flex overflow-hidden">
        <Sidebar
          currentTab={currentTab}
          onSelectTab={setCurrentTab}
          unboundStatus={unboundHealthy}
        />

        <main className="flex-1 overflow-y-auto p-6 md:p-8">
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
        </main>
      </div>

      <LoginModal
        isOpen={showLogin}
        onSuccess={(user) => {
          setCurrentUser(user.username);
          setShowLogin(false);
          loadInitialData();
        }}
      />
    </div>
  );
}

export default App;
