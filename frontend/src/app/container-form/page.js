'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';

export default function ContainerForm() {
  const [form, setForm] = useState({
    name: '',
    image: '',
    cpu: 1,
    memory: 512,
    volume: false,
    volumeSize: 1,
    ports: '',
    envs: '',
    softwares: [],
    script: '',
    gpu: false,
    purpose: '',
    duration: '',
    notify: false
  });

  const router = useRouter();

  const handleChange = (e) => {
    const { name, value, type, checked } = e.target;
    setForm({
      ...form,
      [name]: type === 'checkbox' ? checked : value
    });
  };

  const handleMultiSelect = (e) => {
    const selected = Array.from(e.target.selectedOptions).map(o => o.value);
    setForm({ ...form, softwares: selected });
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    await fetch('/api/container', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form)
    });
    router.push('/success');
  };

  return (
    <form onSubmit={handleSubmit} className="p-6 max-w-xl mx-auto grid gap-4">
      <h1 className="text-xl font-bold">Criar Contêiner</h1>
      <input name="name" placeholder="Nome do Contêiner" onChange={handleChange} className="border p-2" />
      <input name="image" placeholder="Imagem Base (ex: ubuntu, mysql)" onChange={handleChange} className="border p-2" />
      <input name="cpu" type="number" placeholder="vCPU" onChange={handleChange} className="border p-2" />
      <input name="memory" type="number" placeholder="Memória (MB)" onChange={handleChange} className="border p-2" />
      <label><input type="checkbox" name="volume" onChange={handleChange} /> Armazenamento Persistente</label>
      {form.volume && (
        <input name="volumeSize" type="number" placeholder="Tamanho do volume (GB)" onChange={handleChange} className="border p-2" />
      )}
      <input name="ports" placeholder="Portas (ex: 3000:80, 5432:5432)" onChange={handleChange} className="border p-2" />
      <input name="envs" placeholder="Variáveis de Ambiente (ex: NODE_ENV=prod)" onChange={handleChange} className="border p-2" />
      <select multiple name="softwares" onChange={handleMultiSelect} className="border p-2 h-32">
        <option>Node.js</option>
        <option>MySQL</option>
        <option>Redis</option>
        <option>Python</option>
      </select>
      <textarea name="script" placeholder="Script de Inicialização" onChange={handleChange} className="border p-2" />
      <label><input type="checkbox" name="gpu" onChange={handleChange} /> Precisa de GPU</label>
      <input name="purpose" placeholder="Finalidade" onChange={handleChange} className="border p-2" />
      <input name="duration" placeholder="Duração Esperada" onChange={handleChange} className="border p-2" />
      <label><input type="checkbox" name="notify" onChange={handleChange} /> Notificações por e-mail</label>
      <button type="submit" className="bg-green-500 text-white p-2 rounded">Criar Contêiner</button>
    </form>
  );
}