import { describe, expect, it } from 'bun:test'
import AppHeader from './AppHeader.vue'
import ToastHost from './ToastHost.vue'
import ConfirmDialog from './ConfirmDialog.vue'
import Modal from './Modal.vue'
import WelcomeScreen from './WelcomeScreen.vue'
import SourcePanel from './SourcePanel.vue'
import RunParserButton from './RunParserButton.vue'
import CsvPreview from './CsvPreview.vue'
import ExportBar from './ExportBar.vue'
import SkippedSourcesNotice from './SkippedSourcesNotice.vue'
import RulesView from './RulesView.vue'
import RulesTable from './RulesTable.vue'
import RuleSandbox from './RuleSandbox.vue'
import RuleFormModal from './RuleFormModal.vue'

describe('Vue Components (Phase 7 parity)', () => {
  it('exports all shell components', () => {
    expect(AppHeader).toBeDefined()
    expect(ToastHost).toBeDefined()
    expect(ConfirmDialog).toBeDefined()
    expect(Modal).toBeDefined()
  })

  it('exports all welcome screen components', () => {
    expect(WelcomeScreen).toBeDefined()
  })

  it('exports all workspace components', () => {
    expect(SourcePanel).toBeDefined()
    expect(RunParserButton).toBeDefined()
    expect(CsvPreview).toBeDefined()
    expect(ExportBar).toBeDefined()
    expect(SkippedSourcesNotice).toBeDefined()
  })

  it('exports all rules manager components', () => {
    expect(RulesView).toBeDefined()
    expect(RulesTable).toBeDefined()
    expect(RuleSandbox).toBeDefined()
    expect(RuleFormModal).toBeDefined()
  })
})
