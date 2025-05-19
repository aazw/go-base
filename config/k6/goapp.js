import http from 'k6/http';
import { sleep, check } from 'k6';

export const options = {
  vus: 10000,
  duration: '20s',
};

export default function () {
  let res = http.get('http://goapp:8080/health/liveness');
  check(res, { 'status is 200': (res) => res.status === 200 });
  sleep(1);
}
