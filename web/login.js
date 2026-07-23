const error = new URLSearchParams(window.location.search).get('error');
const errorBox = document.querySelector('#loginError');

if (error === 'invalid') {
  errorBox.textContent = '用户名或密码不正确。';
  errorBox.hidden = false;
} else if (error === 'blocked') {
  errorBox.textContent = '登录失败次数过多，请 10 分钟后重试。';
  errorBox.hidden = false;
}
