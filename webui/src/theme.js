import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'

const CustomTheme = definePreset(Aura, {
    semantic: {
        primary: {
            50: '#f0f4ff',
            100: '#e0e7ff',
            200: '#c7d2fe',
            300: '#a5b4fc',
            400: '#818cf8',
            500: '#6366f1',
            600: '#4f46e5',
            700: '#4338ca',
            800: '#3730a3',
            900: '#312e81',
            950: '#1e1b4b'
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
