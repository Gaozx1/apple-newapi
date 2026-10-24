/*!
 * pay-oauth2 前端注入脚本
 * 作用：在用户中心登录页追加一个「SSO / OAuth 2.0 登录」入口，不修改 ls-go 的任何构建产物。
 */
(function () {
  'use strict'

  var BASE = '/oauth2'
  var BUTTON_ID = 'ls-sso-login-btn'
  var FLOAT_ID = 'ls-sso-float'
  var TEXT = '使用 SSO 一键登录'

  // 由 /oauth2/assets/config.js 注入；未加载或未启用时完全不介入页面
  var CONF = window.__LS_SSO__
  if (!CONF || CONF.enabled !== true) {
    return
  }
  if (CONF.login) BASE = String(CONF.login).replace(/\/login$/, '')
  if (CONF.text) TEXT = String(CONF.text)

  function currentRedirect() {
    var path = location.pathname || '/user/'
    if (path.indexOf('/user') !== 0 && path.indexOf('/console') !== 0) {
      path = '/user/'
    }
    return path
  }

  function loginHref() {
    return BASE + '/login?redirect=' + encodeURIComponent(currentRedirect())
  }

  function isLoginView() {
    if (/\/login\b/.test(location.hash || '')) return true
    return !!document.querySelector('input[type="password"]')
  }

  function makeButton() {
    var a = document.createElement('a')
    a.id = BUTTON_ID
    a.href = loginHref()
    a.textContent = TEXT
    a.setAttribute('data-ls-sso', '1')
    a.style.cssText = [
      'display:flex',
      'align-items:center',
      'justify-content:center',
      'gap:8px',
      'width:100%',
      'margin-top:14px',
      'padding:11px 16px',
      'border:1px solid currentColor',
      'border-radius:10px',
      'font-size:14px',
      'font-weight:500',
      'text-decoration:none',
      'opacity:.9',
      'transition:opacity .15s ease'
    ].join(';')
    a.addEventListener('mouseenter', function () {
      a.style.opacity = '1'
    })
    a.addEventListener('mouseleave', function () {
      a.style.opacity = '.9'
    })
    return a
  }

  function mountButton() {
    if (document.getElementById(BUTTON_ID)) return true
    if (!isLoginView()) return false

    var pwd = document.querySelector('input[type="password"]')
    if (!pwd) return false

    // 从密码框向上找「登录卡片」：优先带 login/card 类名的祖先，否则退回最近的 form
    var card = pwd.closest('form') || pwd.closest('[class*="login"]') || null
    var node = pwd
    for (var i = 0; i < 6 && !card; i++) {
      node = node.parentElement
      if (!node) break
      if (/login|card|panel|box/i.test(node.className || '')) card = node
    }
    var host = card || (pwd.parentElement && pwd.parentElement.parentElement)
    if (!host) return false

    host.appendChild(makeButton())
    return true
  }

  function mountFloating() {
    if (document.getElementById(FLOAT_ID)) return
    if (!isLoginView()) return
    var a = document.createElement('a')
    a.id = FLOAT_ID
    a.href = loginHref()
    a.textContent = 'SSO 登录'
    a.style.cssText = [
      'position:fixed',
      'right:20px',
      'bottom:24px',
      'z-index:2147483000',
      'padding:10px 16px',
      'border-radius:999px',
      'background:#2563eb',
      'color:#fff',
      'font:500 13px/1.2 system-ui,sans-serif',
      'text-decoration:none',
      'box-shadow:0 8px 22px rgba(37,99,235,.35)'
    ].join(';')
    document.body.appendChild(a)
  }

  function tick() {
    if (location.pathname.indexOf('/user') !== 0 && location.pathname.indexOf('/console') !== 0) {
      var existing = document.getElementById(BUTTON_ID)
      if (existing && existing.parentNode) existing.parentNode.removeChild(existing)
      return
    }
    var mounted = mountButton()
    if (!mounted && isLoginView()) mountFloating()
    if (mounted) {
      var float = document.getElementById(FLOAT_ID)
      if (float && float.parentNode) float.parentNode.removeChild(float)
    }
  }

  var pending = false
  function schedule() {
    if (pending) return
    pending = true
    setTimeout(function () {
      pending = false
      try {
        tick()
      } catch (e) {
        /* 静默：注入失败不影响原生登录 */
      }
    }, 120)
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', schedule)
  } else {
    schedule()
  }
  window.addEventListener('hashchange', schedule)
  window.addEventListener('popstate', schedule)
  new MutationObserver(schedule).observe(document.documentElement, { childList: true, subtree: true })
  setInterval(schedule, 2500)
})()
