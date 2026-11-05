import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'

const CustomTheme = definePreset(Aura, {
    semantic: {
        primary: {
            50: '#ecfbff',
            100: '#cff4fb',
            200: '#a5e9f5',
            300: '#6ddbec',
            400: '#2fd3ef',
            500: '#12b3d1',
            600: '#0e9bb5',
            700: '#0b8299',
            800: '#0c6a7d',
            900: '#0f5766',
            950: '#073843'
        },
        colorScheme: {
            light: {
                primary: {
                    color: '{primary.600}',
                    contrastColor: '#ffffff',
                    hoverColor: '{primary.700}',
                    activeColor: '{primary.800}'
                },
                surface: {
                    0: '#ffffff',
                    50: '#fafafa',
                    100: '#f5f5f5',
                    200: '#eeeeee',
                    300: '#e0e0e0',
                    400: '#bdbdbd',
                    500: '#9e9e9e',
                    600: '#757575',
                    700: '#616161',
                    800: '#424242',
                    900: '#212121',
                    950: '#0a0a0a'
                },
                text: {
                    color: '#1f2937',
                    hoverColor: '#111827',
                    mutedColor: '#6b7280',
                    highlightColor: '{primary.600}',
                    highlightBackground: '{primary.50}'
                }
            },
            dark: {
                primary: {
                    color: '{primary.400}',
                    contrastColor: '#1f2937',
                    hoverColor: '{primary.300}',
                    activeColor: '{primary.200}'
                },
                // Aura reads the dark surface scale light-to-dark: 0–400 are
                // INKS (text.mutedColor, placeholders, float labels, icons are
                // surface.400–500) and 600–950 are GROUNDS (borders 600–700,
                // hovers 800, panels 900, form fields 950). The scale must stay
                // monotonic: a dark 100–500 (as this preset once had) turns every
                // muted ink — a SelectButton's unselected options, a Tag, an
                // input placeholder — near-black on near-black. The grounds are
                // pinned to the app's own dark tokens (_variables.scss .dark-mode)
                // so PrimeVue panels sit on the same layers as the rest of the UI;
                // 400 is the app's secondary ink (AA on every dark ground), and
                // themeContrast.spec.ts guards both.
                surface: {
                    0: '#ffffff',
                    50: '#f1f4f7',
                    100: '#d7dee6',
                    200: '#bcc6d0',
                    300: '#a3aebb',
                    400: '#8b97a4',
                    500: '#6f7b88',
                    600: '#4a5566',
                    700: '#2c3747',
                    800: '#1c2632',
                    900: '#141b24',
                    950: '#0d1117'
                },
                // Aura's dark text is pure white (surface.0); the app's ink is
                // the softer --app-text-primary, so PrimeVue text matches it.
                text: {
                    color: '{surface.100}',
                    hoverColor: '{surface.50}',
                    mutedColor: '{surface.400}',
                    hoverMutedColor: '{surface.300}'
                }
            }
        }
    },
    components: {
        button: {
            borderRadius: '8px',
            paddingX: '1rem',
            paddingY: '0.625rem'
        },
        card: {
            borderRadius: '12px',
            shadow: '0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px 0 rgba(0, 0, 0, 0.06)'
        }
    }
})

export default CustomTheme
