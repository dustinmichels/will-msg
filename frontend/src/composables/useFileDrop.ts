import { onMounted, onUnmounted, ref } from 'vue'
import { OnFileDrop, OnFileDropOff } from 'wailsjs/runtime'
import { useParseStore } from '@/stores/parse'

export function useFileDrop() {
  const parseStore = useParseStore()
  const isDragging = ref(false)

  onMounted(() => {
    try {
      OnFileDrop((_x: number, _y: number, paths: string[]) => {
        isDragging.value = false
        if (paths && paths.length > 0) {
          parseStore.scanSources(paths)
        }
      }, true)
    } catch (e: unknown) {
      // In non-Wails environment (e.g. headless tests or browser preview)
    }
  })

  onUnmounted(() => {
    try {
      OnFileDropOff()
    } catch (e: unknown) {
      // In non-Wails environment
    }
  })

  return {
    isDragging,
  }
}
