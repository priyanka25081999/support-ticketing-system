import { FormEvent, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { authApi } from '../api/endpoints';
import { useAuth } from '../auth/AuthContext';
import { getErrorMessage } from '../utils';

export function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const from = (useLocation().state as { from?: string } | null)?.from ?? '/dashboard';
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      await login(email, password);
      navigate(from, { replace: true });
    } catch (err) {
      setError(getErrorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="card auth-card" onSubmit={handleSubmit}>
      <h2>Sign in</h2>
      {error && <div className="alert alert-error">{error}</div>}
      <label>
        Email
        <input type="email" required autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} />
      </label>
      <label>
        Password
        <input type="password" required autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} />
      </label>
      <button className="btn btn-primary" disabled={busy}>
        {busy ? 'Signing in…' : 'Sign in'}
      </button>
      <p className="muted">
        New customer? <Link to="/register">Create an account</Link>
      </p>
    </form>
  );
}

export function RegisterPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [form, setForm] = useState({ fullName: '', email: '', password: '' });
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      await authApi.register(form.fullName, form.email, form.password);
      await login(form.email, form.password);
      navigate('/dashboard', { replace: true });
    } catch (err) {
      setError(getErrorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  const update = (field: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm({ ...form, [field]: e.target.value });

  return (
    <form className="card auth-card" onSubmit={handleSubmit}>
      <h2>Create account</h2>
      {error && <div className="alert alert-error">{error}</div>}
      <label>
        Full name
        <input required maxLength={100} value={form.fullName} onChange={update('fullName')} />
      </label>
      <label>
        Email
        <input type="email" required value={form.email} onChange={update('email')} />
      </label>
      <label>
        Password (min 8 characters)
        <input type="password" required minLength={8} maxLength={72} value={form.password} onChange={update('password')} />
      </label>
      <button className="btn btn-primary" disabled={busy}>
        {busy ? 'Creating…' : 'Register'}
      </button>
      <p className="muted">
        Already have an account? <Link to="/login">Sign in</Link>
      </p>
    </form>
  );
}
