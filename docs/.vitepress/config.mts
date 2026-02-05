import { withMermaid } from "vitepress-plugin-mermaid";

// https://vitepress.dev/reference/site-config
export default withMermaid({
  title: "Cloud",
  description: "Documentation for AusOcean's cloud services",
  head: [["link", { rel: "icon", href: "/favicon.ico" }]],
  base: "/",
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: "Home", link: "/" },
      { text: "Documentation", link: "/introduction" },
    ],

    sidebar: [
      {
        base: "/",
        text: "For Users:",
        items: [
          { text: "Introduction", link: "/introduction" },
          {
            text: "OceanBench",
            link: "/oceanbench/oceanbench",
            items: [
              {
                text: "Configuration",
                link: "/oceanbench/device-configuration",
                items: [
                  {
                    text: "Auto Configuration",
                    link: "/oceanbench/autoconfiguration",
                  },
                ],
              },
              {
                text: "Broadcasting",
                link: "oceanbench/broadcast/broadcast",
                items: [
                  {
                    text: "Broadcast Settings",
                    link: "oceanbench/broadcast/broadcast-settings",
                  },
                  {
                    text: "Selecting a Channel",
                    link: "oceanbench/broadcast/selecting-channel",
                  },
                  {
                    text: "Failure Mode",
                    link: "oceanbench/broadcast/failure-mode",
                  },
                ],
              },
              {
                text: "For SuperAdmins",
                link: "oceanbench/super-admins/super-admins.md",
                items: [
                  {
                    text: "TV Overview",
                    link: "oceanbench/super-admins/tv-overview.md",
                  },
                ],
              },
            ],
          },
        ],
      },
      {
        base: "/",
        text: "For Developers",
        items: [
          {
            text: "Netsender",
            link: "netsender/introduction",
            items: [
              {
                text: "Technical Overview",
                link: "netsender/technical-overview",
              },
              {
                text: "Config",
                link: "netsender/config",
              },
              {
                text: "Poll",
                link: "netsender/poll",
              },
              {
                text: "Vars",
                link: "netsender/vars",
              },
              {
                text: "Response Codes",
                link: "netsender/response-codes",
              },
            ],
          },
        ],
      },
    ],

    search: { provider: "local" },

    socialLinks: [
      { icon: "github", link: "https://github.com/ausocean/cloud" },
      { icon: "facebook", link: "https://facebook.com/ausocean" },
      { icon: "x", link: "https://x.com/ausocean" },
      { icon: "youtube", link: "https://youtube.com/ausocean" },
      { icon: "instagram", link: "https://instagram.com/ausocean" },
    ],
  },
  mermaid: {
    // refer https://mermaid.js.org/config/setup/modules/mermaidAPI.html#mermaidapi-configuration-defaults for options
  },
  // optionally set additional config for plugin itself with MermaidPluginConfig
  mermaidPlugin: {
    class: "mermaid my-class", // set additional css classes for parent container
  },
});
