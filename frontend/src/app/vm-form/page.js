'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';

export default function VMForm() {
  const [form, setForm] = useState({
    name: '',
    os: '',
    cpu: 1,
    memory: 1,
    disk: 10,
    diskType: 'SSD',
    network: 'privada',
    ssh: false,
    gpu: false,
    softwares: [],
    script: '',
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
    await fetch('/api/vm', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form)
    });
    router.push('/success');
  };

  return (
    <form onSubmit={handleSubmit} className="p-6 max-w-xl mx-auto grid gap-4">
      <h1 className="text-xl font-bold">Provisionar Máquina Virtual</h1>
      <input name="name" placeholder="Nome da VM" onChange={handleChange} className="border p-2" />
      <select name="os" onChange={handleChange} className="border p-2">
        <option>Escolha o SO</option>
        <option>Ubuntu 22.04</option>
        <option>Debian</option>
        <option>Windows Server</option>
      </select>
      <input name="cpu" type="number" placeholder="vCPU" onChange={handleChange} className="border p-2" />
      <input name="memory" type="number" placeholder="Memória (GB)" onChange={handleChange} className="border p-2" />
      <input name="disk" type="number" placeholder="Disco (GB)" onChange={handleChange} className="border p-2" />
      <select name="diskType" onChange={handleChange} className="border p-2">
        <option>SSD</option>
        <option>SATA</option>
        <option>NVMe</option>
      </select>
      <select name="network" onChange={handleChange} className="border p-2">
        <option value="privada">Rede Privada</option>
        <option value="publica">Rede Pública</option>
      </select>
      <label><input type="checkbox" name="ssh" onChange={handleChange} /> Acesso via SSH</label>
      <label><input type="checkbox" name="gpu" onChange={handleChange} /> Requer GPU</label>
      <select multiple name="softwares" onChange={handleMultiSelect} className="border p-2 h-32">
        <option>Docker</option>
        <option>MySQL</option>
        <option>Jupyter</option>
        <option>VS Code Server</option>
      </select>
      <textarea name="script" placeholder="Script de Provisionamento" onChange={handleChange} className="border p-2" />
      <input name="purpose" placeholder="Finalidade" onChange={handleChange} className="border p-2" />
      <input name="duration" placeholder="Duração Esperada" onChange={handleChange} className="border p-2" />
      <label><input type="checkbox" name="notify" onChange={handleChange} /> Notificações por e-mail</label>
      <button type="submit" className="bg-blue-500 text-white p-2 rounded">Criar VM</button>
    </form>
  );
}
