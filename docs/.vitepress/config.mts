import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Pruvon Docs',
  description: 'Documentation for running and operating Pruvon on a Dokku host.',
  lastUpdated: true,
  themeConfig: {
    nav: [
      { text: 'Install', link: '/install' },
      { text: 'Configuration', link: '/configuration' },
      { text: 'Security', link: '/security' },
      { text: 'Behind Proxy', link: '/behind-proxy' },
    ],
    sidebar: [
      {
        text: 'Getting Started',
        items: [
          { text: 'Overview', link: '/' },
          { text: 'Install', link: '/install' },
          { text: 'Configuration', link: '/configuration' },
          { text: 'Security', link: '/security' },
          { text: 'Behind Proxy', link: '/behind-proxy' },
        ],
      },
    ],
    search: {
      provider: 'local',
    },
    socialLinks: [{ icon: 'github', link: 'https://github.com/pruvon/pruvon' }],
  },
})
