import { create } from 'zustand';

interface UIState {
  selectedStageId: string | null;
  setSelectedStage: (stageId: string | null) => void;

  auditDrawerOpen: boolean;
  toggleAuditDrawer: () => void;
  setAuditDrawerOpen: (open: boolean) => void;

  questionPanelOpen: boolean;
  toggleQuestionPanel: () => void;

  activeTab: 'overview' | 'artifacts' | 'output' | 'gate' | 'revisions' | 'audit';
  setActiveTab: (tab: UIState['activeTab']) => void;

  rawPaneOpen: boolean;
  setRawPaneOpen: (open: boolean) => void;
}

export const useUIStore = create<UIState>((set) => ({
  selectedStageId: null,
  setSelectedStage: (stageId) => set({ selectedStageId: stageId }),

  auditDrawerOpen: false,
  toggleAuditDrawer: () => set((s) => ({ auditDrawerOpen: !s.auditDrawerOpen })),
  setAuditDrawerOpen: (open) => set({ auditDrawerOpen: open }),

  questionPanelOpen: false,
  toggleQuestionPanel: () => set((s) => ({ questionPanelOpen: !s.questionPanelOpen })),

  activeTab: 'overview',
  setActiveTab: (tab) => set({ activeTab: tab }),

  rawPaneOpen: false,
  setRawPaneOpen: (open) => set({ rawPaneOpen: open }),
}));