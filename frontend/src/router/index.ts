import { createRouter, createWebHistory } from "vue-router";

import AlertsPage from "../pages/AlertsPage.vue";
import HostsPage from "../pages/HostsPage.vue";
import OverviewPage from "../pages/OverviewPage.vue";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
      name: "overview",
      component: OverviewPage,
    },
    {
      path: "/hosts",
      name: "hosts",
      component: HostsPage,
    },
    {
      path: "/alerts",
      name: "alerts",
      component: AlertsPage,
    },
    {
      path: "/:pathMatch(.*)*",
      redirect: "/",
    },
  ],
});
