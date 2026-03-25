import { withMermaid } from 'vitepress-plugin-mermaid'
import { fileURLToPath, URL } from 'node:url'

// https://vitepress.dev/reference/site-config
export default withMermaid({
    title: 'Network Operator',
    description: 'Cloud Native Network Device Provisioning',
    base: "/network-operator/",
    head: [
        [
            'link',
            {
                rel: 'icon',
                href: 'https://raw.githubusercontent.com/ironcore-dev/ironcore/refs/heads/main/docs/assets/logo_borderless.svg',
            },
        ],
    ],
    vite: {
        resolve: {
            alias: [
                {
                    find: /^.*\/VPFooter\.vue$/,
                    replacement: fileURLToPath(new URL('./theme/components/VPFooter.vue', import.meta.url)),
                },
            ],
        },
    },
    themeConfig: {
        // https://vitepress.dev/reference/default-theme-config
        nav: [
            { text: 'Home', link: '/' },
            {
                text: 'Documentation',
                items: [
                    { text: 'Overview', link: '/overview' },
                    { text: 'Concepts', link: '/concepts/' },
                    { text: 'Tutorials', link: '/tutorials/' },
                    { text: 'API References', link: '/api-reference/' },
                ],
            },
            {
                text: 'Projects',
                items: [
                    { text: 'ApeiroRA', link: 'https://apeirora.eu/' },
                    { text: 'IronCore', link: 'https://ironcore.dev/' },
                    {
                        text: 'CobaltCore',
                        link: 'https://cobaltcore-dev.github.io/docs/',
                    },
                ],
            },
        ],

        editLink: {
            pattern: 'https://github.com/ironcore-dev/network-operator/blob/main/docs/:path',
            text: 'Edit this page on GitHub',
        },

        logo: {
            src: 'https://raw.githubusercontent.com/ironcore-dev/ironcore/refs/heads/main/docs/assets/logo_borderless.svg',
            width: 24,
            height: 24,
        },

        search: {
            provider: 'local',
        },

        sidebar: [
            {
                text: 'Overview',
                items: [
                    { text: 'Index', link: '/overview/' },
                    { text: 'Architecture', link: '/architecture-overview' },
                ],
            },
            {
                text: 'Concepts',
                items: [
                    { text: 'Index', link: '/concepts/' },
                    { text: 'Pausing Reconciliation', link: '/concepts/pausing' },
                ],
            },
            {
                text: 'Concepts',
                items: [
                    { text: 'Index', link: '/concepts/' },
                    { text: 'Pausing Reconciliation', link: '/concepts/pausing' },
                    { text: 'Numbered Resources', link: '/concepts/numbered-resources' },
                ],
            },
            {
                text: 'Tutorials',
                items: [
                    { text: 'Index', link: '/tutorials/' },
                    { text: 'EVPN/VXLAN Fabric', link: '/tutorials/evpn-vxlan-fabric' }
                ],
            },
            {
                text: 'API References',
                items: [{ text: 'Index', link: '/api-reference/' }],
            },
        ],

        socialLinks: [{ icon: 'github', link: 'https://github.com/ironcore-dev/network-operator' }],
    },
})
