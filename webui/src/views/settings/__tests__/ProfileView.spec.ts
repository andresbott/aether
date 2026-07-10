import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ProfileView from '@/views/settings/ProfileView.vue'

describe('ProfileView', () => {
    it('renders the profile placeholder', () => {
        const w = mount(ProfileView)
        expect(w.text()).toContain('Profile')
        expect(w.text().toLowerCase()).toContain('placeholder')
    })
})
