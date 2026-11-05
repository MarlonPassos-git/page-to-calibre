import './style.css';
import { mount } from './main';
const root = document.querySelector<HTMLElement>('#app');
if (root) mount(root);
