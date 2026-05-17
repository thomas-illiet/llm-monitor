import { createVuetify } from 'vuetify'
import {
  VAlert,
  VApp,
  VBtn,
  VBtnToggle,
  VCard,
  VCardActions,
  VCardText,
  VCardTitle,
  VChip,
  VDataTable,
  VDataTableServer,
  VDialog,
  VDivider,
  VList,
  VListItem,
  VListItemTitle,
  VMain,
  VMenu,
  VProgressCircular,
  VSelect,
  VTable,
  VTextField,
  VTooltip
} from 'vuetify/components'
import { Ripple } from 'vuetify/directives'

export default createVuetify({
  components: {
    VAlert,
    VApp,
    VBtn,
    VBtnToggle,
    VCard,
    VCardActions,
    VCardText,
    VCardTitle,
    VChip,
    VDataTable,
    VDataTableServer,
    VDialog,
    VDivider,
    VList,
    VListItem,
    VListItemTitle,
    VMain,
    VMenu,
    VProgressCircular,
    VSelect,
    VTable,
    VTextField,
    VTooltip
  },
  directives: {
    Ripple
  },
  theme: {
    defaultTheme: 'openaiLight',
    themes: {
      openaiLight: {
        dark: false,
        colors: {
          background: '#f7f7f4',
          surface: '#ffffff',
          primary: '#10a37f',
          secondary: '#6b7280',
          error: '#b42318',
          info: '#2563eb',
          success: '#0f8f6f',
          warning: '#b7791f'
        }
      },
      openaiDark: {
        dark: true,
        colors: {
          background: '#202123',
          surface: '#2f2f33',
          primary: '#10a37f',
          secondary: '#8e8ea0',
          error: '#ef4444',
          info: '#3b82f6',
          success: '#10a37f',
          warning: '#f59e0b'
        }
      }
    }
  },
  defaults: {
    VCard: {
      elevation: 0,
      rounded: 'lg'
    },
    VBtn: {
      rounded: 'lg'
    }
  }
})
