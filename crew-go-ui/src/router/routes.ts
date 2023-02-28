import { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    component: () => import('layouts/MainLayout.vue'),
    children: [{ path: '', name: 'home', component: () => import('pages/IndexPage.vue') }]
  },
  {
    path: '/task-group',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      { path: ':taskGroupId', name: 'task_group', component: () => import('src/pages/TaskGroupPage.vue') },
      { path: ':taskGroupId/task/:taskId', name: 'task', component: () => import('src/pages/TaskPage.vue') }
    ],
    meta: {
      requiresAuth: true
    }
  },

  // Always leave this as last one,
  // but you can also remove it
  {
    path: '/:catchAll(.*)*',
    component: () => import('pages/ErrorNotFound.vue')
  }
]

export default routes
