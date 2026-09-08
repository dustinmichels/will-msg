import { describe, expect, it, beforeEach } from 'bun:test'
import { setActivePinia, createPinia } from 'pinia'
import { useNavigationStore } from './navigation'
import { useParseStore } from './parse'
import { useRulesStore } from './rules'
import { scanner, config } from 'wailsjs/go/models'

describe('Navigation Store & Screen Switching', () => {
  beforeEach(() => {
    setActivePinia(createPinia())

    const globalObj = globalThis as unknown as {
      window: {
        go: {
          appservice: {
            Service: {
              GetRules: () => Promise<config.RuleConfig>
              SetRulesDirty: (dirty: boolean) => Promise<void>
            }
          }
        }
      }
    }

    globalObj.window = {
      go: {
        appservice: {
          Service: {
            GetRules: async () => config.RuleConfig.createFrom({
              version: 1,
              enable_heuristics: true,
              default_label: 'other',
              labels: [],
              rules: [],
            }),
            SetRulesDirty: async () => {},
          },
        },
      },
    }
  })

  it('computes view as welcome when no sources are loaded', () => {
    const navStore = useNavigationStore()
    const parseStore = useParseStore()

    expect(parseStore.sources.length).toBe(0)
    expect(navStore.view).toBe('welcome')
  })

  it('computes view as workspace when sources are loaded', () => {
    const navStore = useNavigationStore()
    const parseStore = useParseStore()

    parseStore.sources = [scanner.MessageSource.createFrom({ path: 'test.msg', in_zip: false, zip_path: '', display_name: 'test.msg' })]
    expect(navStore.view).toBe('workspace')
  })

  it('computes view as rules when activeView is rules', async () => {
    const navStore = useNavigationStore()
    const parseStore = useParseStore()

    parseStore.sources = [scanner.MessageSource.createFrom({ path: 'test.msg', in_zip: false, zip_path: '', display_name: 'test.msg' })]
    await navStore.openRules()

    expect(navStore.view).toBe('rules')
    expect(navStore.isRulesOpen).toBe(true)
  })

  it('closes rules immediately when store is not dirty', async () => {
    const navStore = useNavigationStore()
    const rulesStore = useRulesStore()
    await rulesStore.loadRules()

    await navStore.openRules()
    expect(navStore.isRulesOpen).toBe(true)

    const closed = await navStore.closeRules()
    expect(closed).toBe(true)
    expect(navStore.isRulesOpen).toBe(false)
  })

  it('guards dirty rules on exit and cancels exit if user rejects discard', async () => {
    const navStore = useNavigationStore()
    const rulesStore = useRulesStore()
    await rulesStore.loadRules()

    await navStore.openRules()
    await rulesStore.addRule({ pattern: 'DIRTY', label: 'other' })
    expect(rulesStore.isDirty).toBe(true)

    // User rejects discard
    const confirmFn = async () => false
    const closed = await navStore.closeRules(confirmFn)

    expect(closed).toBe(false)
    expect(navStore.isRulesOpen).toBe(true)
    expect(rulesStore.isDirty).toBe(true)
  })

  it('guards dirty rules on exit, discards changes, and closes when user confirms', async () => {
    const navStore = useNavigationStore()
    const rulesStore = useRulesStore()
    await rulesStore.loadRules()

    await navStore.openRules()
    await rulesStore.addRule({ pattern: 'DIRTY', label: 'other' })
    expect(rulesStore.isDirty).toBe(true)

    // User confirms discard
    const confirmFn = async () => true
    const closed = await navStore.closeRules(confirmFn)

    expect(closed).toBe(true)
    expect(navStore.isRulesOpen).toBe(false)
    expect(rulesStore.isDirty).toBe(false)
  })
})
