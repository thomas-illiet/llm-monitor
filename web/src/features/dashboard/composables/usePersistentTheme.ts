import { computed, readonly, shallowRef, watch } from 'vue'
import { useTheme } from 'vuetify'

type ThemeName = 'openaiLight' | 'openaiDark'

const STORAGE_KEY = 'llm-monitor-theme'
const LIGHT_THEME: ThemeName = 'openaiLight'
const DARK_THEME: ThemeName = 'openaiDark'

/** Reads the persisted theme name, defaulting to the light dashboard theme. */
function readStoredTheme(): ThemeName {
  const stored = window.localStorage.getItem(STORAGE_KEY)
  return stored === DARK_THEME ? DARK_THEME : LIGHT_THEME
}

/** Synchronizes Vuetify theme state with local storage and document colors. */
export function usePersistentTheme() {
  const theme = useTheme()
  const selectedTheme = shallowRef<ThemeName>(readStoredTheme())
  const isDark = computed(() => selectedTheme.value === DARK_THEME)

  watch(selectedTheme, (name) => {
    const dark = name === DARK_THEME
    const bg = dark ? '#202123' : '#f7f7f4'

    theme.global.name.value = name
    window.localStorage.setItem(STORAGE_KEY, name)
    document.documentElement.dataset.theme = dark ? 'dark' : 'light'
    document.documentElement.style.background = bg
    document.body.style.background = bg
  }, { immediate: true })

  /** Toggles between the configured light and dark dashboard themes. */
  function toggleTheme() {
    selectedTheme.value = isDark.value ? LIGHT_THEME : DARK_THEME
  }

  return {
    isDark: readonly(isDark),
    toggleTheme
  }
}
