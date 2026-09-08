import { defineStore } from 'pinia'
import { computed } from 'vue'
import { useParseStore } from '@/stores/parse'
import { useRulesStore } from '@/stores/rules'
import { confirm as defaultConfirm } from '@/lib/confirm'

export type AppView = 'welcome' | 'workspace' | 'rules'

export interface NavigationState {
  activeView: 'main' | 'rules'
}

export const useNavigationStore = defineStore('navigation', {
  state: (): NavigationState => ({
    activeView: 'main',
  }),

  getters: {
    view(state): AppView {
      if (state.activeView === 'rules') {
        return 'rules'
      }
      const parseStore = useParseStore()
      if (parseStore.hasSources) {
        return 'workspace'
      }
      return 'welcome'
    },
    isRulesOpen(state): boolean {
      return state.activeView === 'rules'
    },
  },

  actions: {
    async openRules(): Promise<void> {
      const rulesStore = useRulesStore()
      if (!rulesStore.workingConfig) {
        try {
          await rulesStore.loadRules()
        } catch (err: unknown) {
          console.error('Failed to load rules on open:', err)
        }
      }
      this.activeView = 'rules'
    },

    async closeRules(
      confirmFn?: () => Promise<boolean>
    ): Promise<boolean> {
      const rulesStore = useRulesStore()
      if (rulesStore.isDirty) {
        const shouldDiscard = confirmFn
          ? await confirmFn()
          : await defaultConfirm({
              title: 'Discard Unsaved Changes?',
              message: 'You have unsaved changes in Rules & Labels. If you leave now, these changes will be lost.',
              confirmText: 'Discard Changes',
              cancelText: 'Keep Editing',
              danger: true,
            })

        if (!shouldDiscard) {
          return false
        }

        await rulesStore.discardChanges()
      }

      this.activeView = 'main'
      return true
    },
  },
})
