import { createRouter, createWebHistory } from "vue-router";

import AlertsPage from "../pages/AlertsPage.vue";
import HostDetailPage from "../pages/HostDetailPage.vue";
import HostsPage from "../pages/HostsPage.vue";
import OverviewPage from "../pages/OverviewPage.vue";
import StatusPage from "../pages/StatusPage.vue";

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
      path: "/hosts/:instance",
      name: "host-detail",
      component: HostDetailPage,
      props: true,
    },
    {
      path: "/status",
      name: "status",
      component: StatusPage,
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
