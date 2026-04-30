import { createRouter, createWebHistory } from "vue-router";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
      name: "overview",
      component: () => import("../pages/OverviewPage.vue"),
    },
    {
      path: "/hosts",
      name: "hosts",
      component: () => import("../pages/HostsPage.vue"),
    },
    {
      path: "/hosts/:instance",
      name: "host-detail",
      component: () => import("../pages/HostDetailPage.vue"),
      props: true,
    },
    {
      path: "/status",
      name: "status",
      component: () => import("../pages/StatusPage.vue"),
    },
    {
      path: "/alerts",
      name: "alerts",
      component: () => import("../pages/AlertsPage.vue"),
    },
    {
      path: "/:pathMatch(.*)*",
      redirect: "/",
    },
  ],
});
