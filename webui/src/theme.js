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
                surface: {
                    0: '#ffffff',
                    50: '#f9fafb',
                    100: '#374151',
                    200: '#333a47',
                    300: '#2f3541',
                    400: '#2b303b',
                    500: '#272c35',
                    600: '#1f2937',
                    700: '#1a202c',
                    800: '#151a23',
                    900: '#10141b',
                    950: '#0b0e13'
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
